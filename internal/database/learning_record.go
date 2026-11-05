package database

import (
	"context"
	"errors"

	"github.com/UnlockedLabs/unlocked-contracts/identity"
	"github.com/UnlockedLabs/unlocked-learning-record/internal/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Learning record entries and drafts. The writes take the request's context,
// as core's srv.WithUserContext(r) did, so the audit hooks and the explicit
// actor reads in DeleteEntry and UpsertDraft record the caller as the row's
// update user.

func (db *DB) withFacilityName() *gorm.DB {
	return db.Model(&models.LearningRecordEntry{}).
		Select("learning_record_entries.*, COALESCE(facilities.name, '') AS facility_name").
		Joins("LEFT JOIN facilities ON facilities.id = learning_record_entries.facility_id")
}

func (db *DB) GetEntries(userID uint) ([]models.LearningRecordEntry, error) {
	entries := make([]models.LearningRecordEntry, 0)
	if err := db.withFacilityName().
		Where("learning_record_entries.user_id = ? AND learning_record_entries.is_draft = false", userID).
		Order("learning_record_entries.created_at DESC").
		Find(&entries).Error; err != nil {
		return nil, newGetRecordsDBError(err, "learning_record_entries")
	}
	return entries, nil
}

func (db *DB) CreateEntry(ctx context.Context, entry *models.LearningRecordEntry) error {
	if err := db.WithContext(ctx).Create(entry).Error; err != nil {
		return newCreateDBError(err, "learning_record_entries")
	}
	return nil
}

func (db *DB) UpdateEntry(ctx context.Context, entry *models.LearningRecordEntry) error {
	result := db.WithContext(ctx).Model(entry).
		Where("id = ? AND user_id = ?", entry.ID, entry.UserID).
		Updates(entryColumns(entry))
	if result.Error != nil {
		return newUpdateDBError(result.Error, "learning_record_entries")
	}
	return nil
}

// DeleteEntry soft-deletes the user's entry and adds a learning_record_deleted
// row to core's user_account_history, naming the caller, in one transaction.
func (db *DB) DeleteEntry(ctx context.Context, id, userID uint) error {
	actorID := userID
	if c, ok := identity.FromContext(ctx); ok {
		actorID = c.UserID
	}
	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&models.LearningRecordEntry{}).
			Where("id = ? AND user_id = ?", id, userID).
			Update("update_user_id", actorID).Error; err != nil {
			return newUpdateDBError(err, "learning_record_entries")
		}
		if err := tx.Where("id = ? AND user_id = ?", id, userID).
			Delete(&models.LearningRecordEntry{}).Error; err != nil {
			return newDeleteDBError(err, "learning_record_entries")
		}
		history := models.NewUserAccountHistory(userID, models.LearningRecordDeleted, &actorID)
		if err := tx.Create(history).Error; err != nil {
			return newCreateDBError(err, "user_account_history")
		}
		return nil
	})
}

// GetDraft returns the most recently updated draft for the user, or nil.
func (db *DB) GetDraft(userID uint) (*models.LearningRecordEntry, error) {
	var draft models.LearningRecordEntry
	err := db.withFacilityName().
		Where("learning_record_entries.user_id = ? AND learning_record_entries.is_draft = true", userID).
		Order("learning_record_entries.updated_at DESC").
		First(&draft).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, newGetRecordsDBError(err, "learning_record_entries")
	}
	return &draft, nil
}

// UpsertDraft inserts or updates a draft row keyed on (user_id, client_id).
// Its ON CONFLICT target doesn't match core's migration 00069's partial
// unique index, so it 500s on every call, in production too. It moved from
// core unchanged; see README.md's Tests section.
func (db *DB) UpsertDraft(ctx context.Context, draft *models.LearningRecordEntry) error {
	draft.IsDraft = true
	updateMap := draftColumns(draft)
	updateMap["is_draft"] = true
	updateMap["updated_at"] = gorm.Expr("NOW()")
	if c, ok := identity.FromContext(ctx); ok {
		updateMap["update_user_id"] = c.UserID
	}
	if err := db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "user_id"}, {Name: "client_id"}},
		DoUpdates: clause.Assignments(updateMap),
	}).Create(draft).Error; err != nil {
		return newCreateDBError(err, "learning_record_entries")
	}
	return nil
}

func (db *DB) DeleteDraft(ctx context.Context, userID uint, clientID string) error {
	if err := db.WithContext(ctx).Where("user_id = ? AND client_id = ? AND is_draft = true", userID, clientID).
		Delete(&models.LearningRecordEntry{}).Error; err != nil {
		return newDeleteDBError(err, "learning_record_entries")
	}
	return nil
}

// entryColumns holds the achievement fields shared by drafts and committed
// entries. The draft-only wizard state (step_index, ui_phase,
// editing_entry_id) is excluded so editing a committed entry cannot reset it.
func entryColumns(e *models.LearningRecordEntry) map[string]any {
	return map[string]any{
		"program_name": e.ProgramName,
		"facility_id":  e.FacilityID, "facility_other": e.FacilityOther,
		"completion_date": e.CompletionDate, "confidence": e.Confidence,
		"summary": e.Summary, "top_skills": e.TopSkills,
		"barrier_to_completion": e.BarrierToCompletion, "goal_connection": e.GoalConnection,
		"pride": e.Pride, "standout_moment": e.StandoutMoment,
		"advice_to_peer": e.AdviceToPeer, "challenge_toggle": e.ChallengeToggle,
		"challenge_text": e.ChallengeText, "skill_tags_before": e.SkillTagsBefore,
		"skill_tags_after": e.SkillTagsAfter, "skill_reflection": e.SkillReflection,
		"growth_reflection": e.GrowthReflection, "support_selections": e.SupportSelections,
		"next_step_selections": e.NextStepSelections,
	}
}

// draftColumns adds the draft-only wizard state to the shared achievement
// fields; use it for draft upserts only.
func draftColumns(e *models.LearningRecordEntry) map[string]any {
	columns := entryColumns(e)
	columns["step_index"] = e.StepIndex
	columns["ui_phase"] = e.UiPhase
	columns["editing_entry_id"] = e.EditingEntryID
	return columns
}

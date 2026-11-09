package models

// LearningRecordEntry is a resident's achievement, or the in-progress draft
// of one (IsDraft). It lives in the learning_record schema.
type LearningRecordEntry struct {
	DatabaseFields
	UserID              uint        `gorm:"not null" json:"user_id"`
	ClientID            string      `gorm:"not null" json:"client_id"`
	IsDraft             bool        `gorm:"not null;default:false" json:"is_draft"`
	StepIndex           int         `json:"step_index"`
	UiPhase             string      `json:"ui_phase"`
	EditingEntryID      *uint       `json:"editing_entry_id"`
	ProgramName         string      `json:"program_name"`
	FacilityID          *uint       `json:"facility_id"`
	FacilityOther       string      `json:"facility_other"`
	FacilityName        string      `gorm:"->;-:migration" json:"facility_name"`
	CompletionDate      string      `json:"completion_date"`
	Confidence          string      `json:"confidence"`
	Summary             string      `json:"summary"`
	TopSkills           StringSlice `gorm:"type:text" json:"top_skills"`
	BarrierToCompletion string      `json:"barrier_to_completion"`
	GoalConnection      string      `json:"goal_connection"`
	Pride               string      `json:"pride"`
	StandoutMoment      string      `json:"standout_moment"`
	AdviceToPeer        string      `json:"advice_to_peer"`
	ChallengeToggle     *string     `json:"challenge_toggle"`
	ChallengeText       string      `json:"challenge_text"`
	SkillTagsBefore     StringSlice `gorm:"type:text" json:"skill_tags_before"`
	SkillTagsAfter      StringSlice `gorm:"type:text" json:"skill_tags_after"`
	SkillReflection     string      `json:"skill_reflection"`
	GrowthReflection    string      `json:"growth_reflection"`
	SupportSelections   StringSlice `gorm:"type:text" json:"support_selections"`
	NextStepSelections  StringSlice `gorm:"type:text" json:"next_step_selections"`
}

func (LearningRecordEntry) TableName() string { return "learning_record_entries" }

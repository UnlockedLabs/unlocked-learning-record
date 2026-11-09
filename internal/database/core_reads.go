package database

import "github.com/UnlockedLabs/unlocked-learning-record/internal/models"

// Learning Record's reads of core's facilities table, for the
// achievement-location picker and to check an entry's facility_id. Its one
// write to a core table, the user_account_history row an entry delete adds,
// is in DeleteEntry, in the delete's transaction.

/*
GetLocationFacility resolves a facility id used as an achievement location,
returning the facility's name and whether it exists at all. The lookup is
unscoped on purpose: an achievement can point at a facility that has since
been soft-deleted, and the foreign key still holds, so rejecting those would
block edits to older records. ok is false only for an id that is not a
facility.
*/
func (db *DB) GetLocationFacility(id uint) (string, bool, error) {
	var names []string
	if err := db.Unscoped().Model(&models.Facility{}).
		Where("id = ?", id).
		Limit(1).
		Pluck("name", &names).Error; err != nil {
		return "", false, newGetRecordsDBError(err, "facilities")
	}
	if len(names) == 0 {
		return "", false, nil
	}
	return names[0], true, nil
}

// GetAllFacilitiesOrdered is every facility Learning Record's location picker
// offers, alphabetical. GORM's default scope excludes a soft-deleted one.
func (db *DB) GetAllFacilitiesOrdered() ([]models.Facility, error) {
	var facilities []models.Facility
	if err := db.Order("name").Find(&facilities).Error; err != nil {
		return nil, newGetRecordsDBError(err, "facilities")
	}
	return facilities, nil
}

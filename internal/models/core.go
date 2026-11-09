package models

import "time"

// The rows of core's tables that Learning Record touches. Its role may read
// facilities and add user_account_history rows, nothing else (see the
// migration).

// Facility is the read Learning Record makes of core's facilities table: the
// id and name shown in the achievement-location picker. It carries
// DatabaseFields' DeletedAt so GORM's default soft-delete scope excludes a
// deleted facility from that list, the same as core's models.Facility does.
type Facility struct {
	DatabaseFields
	Name string `gorm:"size:255;not null"`
}

func (Facility) TableName() string { return "facilities" }

// UserAccountHistory is a row of core's user_account_history table. Learning
// Record adds one when a resident deletes an entry.
type UserAccountHistory struct {
	UserID           uint                  `json:"user_id" gorm:"primaryKey"`
	AdminID          *uint                 `json:"admin_id"`
	Action           ActivityHistoryAction `json:"action" gorm:"size:255;primaryKey"`
	CreatedAt        time.Time             `json:"created_at" gorm:"primaryKey"`
	AttendanceStatus string                `json:"attendance_status" gorm:"size:27"`
}

func (UserAccountHistory) TableName() string { return "user_account_history" }

// ActivityHistoryAction is a user_account_history action.
type ActivityHistoryAction string

// LearningRecordDeleted is the action core's account history shows for a
// deleted entry. Core has the same value.
const LearningRecordDeleted ActivityHistoryAction = "learning_record_deleted"

func NewUserAccountHistory(userID uint, action ActivityHistoryAction, adminID *uint) *UserAccountHistory {
	return &UserAccountHistory{
		UserID:    userID,
		AdminID:   adminID,
		Action:    action,
		CreatedAt: time.Now(),
	}
}

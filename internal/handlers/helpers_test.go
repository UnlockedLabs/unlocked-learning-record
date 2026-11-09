package handlers

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/UnlockedLabs/unlocked-learning-record/internal/database"
	"github.com/UnlockedLabs/unlocked-learning-record/internal/models"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// newTestDB returns an in-memory database with Learning Record's table and
// the part of core's tables it reads and writes, closed when the test ends.
// Like core's tests, it runs on SQLite, which has no schemas or roles.
func newTestDB(t *testing.T) *database.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:%s_%d?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"), time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{DisableForeignKeyConstraintWhenMigrating: true})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	for _, stmt := range []string{
		`CREATE TABLE facilities (id INTEGER PRIMARY KEY, created_at DATETIME, updated_at DATETIME, deleted_at DATETIME,
			create_user_id INTEGER, update_user_id INTEGER, name TEXT NOT NULL)`,
		`CREATE TABLE users (id INTEGER PRIMARY KEY, username TEXT NOT NULL)`,
		`CREATE TABLE user_account_history (user_id INTEGER NOT NULL, admin_id INTEGER, action TEXT NOT NULL,
			program_classes_history_id INTEGER, facility_id INTEGER, created_at DATETIME NOT NULL,
			attendance_status TEXT, class_name TEXT, session_date DATE, PRIMARY KEY (user_id, action, created_at))`,
	} {
		if err := db.Exec(stmt).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := db.AutoMigrate(&models.LearningRecordEntry{}); err != nil {
		t.Fatal(err)
	}
	return database.NewDB(db)
}

// northID is the resident's facility, the one core's test database added
// after its default one.
const northID = 2

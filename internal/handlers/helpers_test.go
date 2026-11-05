package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/UnlockedLabs/unlocked-contracts/features"
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

// Core's facilities, as its test database had them: its default one, then
// the ones the tests add.
const (
	northID = 2
	southID = 3
	// unknownFacilityID is a facility core can't answer about.
	unknownFacilityID = 9
)

// startFakeCore answers like core's /internal/v1/features, which
// features.Checker asks: every feature is on at North, every one but
// learning_record at South, and core fails for unknownFacilityID.
func startFakeCore(t *testing.T) string {
	t.Helper()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/internal/v1/features" {
			http.NotFound(w, r)
			return
		}
		id, _ := strconv.Atoi(r.URL.Query().Get("facility_id"))
		var on []features.Access
		switch id {
		case northID:
			on = features.All
		case southID:
			on = slices.DeleteFunc(slices.Clone(features.All), func(f features.Access) bool { return f == features.LearningRecord })
		case unknownFacilityID:
			http.Error(w, `{"message":"error getting facility_feature_flags","data":null}`, http.StatusInternalServerError)
			return
		default:
			http.Error(w, `{"message":"facilities not found","data":null}`, http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"message": "Data fetched successfully",
			"data":    map[string]any{"facility_id": id, "features": on},
		})
	}))
	t.Cleanup(ts.Close)
	return ts.URL
}

package handlers

import (
	"bytes"
	"encoding/json"
	"flag"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/UnlockedLabs/unlocked-contracts/features"
	"github.com/UnlockedLabs/unlocked-contracts/identity"
	"github.com/UnlockedLabs/unlocked-contracts/users"
	"github.com/UnlockedLabs/unlocked-learning-record/internal/database"
	"github.com/UnlockedLabs/unlocked-learning-record/internal/models"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// These tests pin what every Learning Record route answers: the status and
// the JSON, who may call it, and the two behaviors invisible in a JSON
// response body (a soft-deleted facility is excluded from the location
// picker, and deleting an entry records who deleted it). They came from
// core's tests/integration/learning_record_routes_test.go (EN-195) with its
// golden files in testdata/learning_record unchanged, so passing them shows
// the routes answer exactly as they did in core (EN-208).
//
// Regenerate the golden files after an intended change with:
//
//	go test ./internal/handlers -run 'TestLearningRecord' -update

var updateGolden = flag.Bool("update", false, "rewrite the golden files in testdata/learning_record")

// testClaimsHeader carries the caller's identity.Claims as JSON, in place of
// the login session the tests don't have, as core's X-Test-Claims did.
const testClaimsHeader = "X-Test-Claims"

// The resident the tests call as. The ID is the one core's test database
// gave them, after its default user (1); the golden files name it.
var residentClaims = &identity.Claims{UserID: 2, Username: "jdoe", Role: users.Student, FacilityID: northID, FacilityName: "North",
	FeatureAccess: []features.Access{features.LearningRecord}}

// withoutLearningRecord is c with the learning_record feature off at its
// facility: the same claims, without it in FeatureAccess.
func withoutLearningRecord(c *identity.Claims) *identity.Claims {
	off := *c
	off.FeatureAccess = nil
	return &off
}

// newRouteTest starts the routes on a test database seeded as core's test
// seeded it.
func newRouteTest(t *testing.T) (*httptest.Server, *database.DB) {
	t.Helper()
	db := newTestDB(t)
	seedCore(t, db)
	srv := &server{db: db}
	ts := httptest.NewServer(newRouter(srv.routes(), testLogin))
	t.Cleanup(ts.Close)
	return ts, db
}

// testLogin passes the caller in testClaimsHeader, as identity.SessionChecker
// passes a logged-in caller.
func testLogin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var c identity.Claims
		if err := json.Unmarshal([]byte(r.Header.Get(testClaimsHeader)), &c); err != nil {
			http.Error(w, "test claims: "+err.Error(), http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r.WithContext(identity.NewContext(r.Context(), &c)))
	})
}

// seedCore adds what core's test added: its default facility, North and
// South, a soft-deleted Retired facility (to prove the location picker
// excludes it), core's default user and the resident.
func seedCore(t *testing.T, db *database.DB) {
	t.Helper()
	now := time.Now()
	for _, stmt := range []struct {
		sql  string
		args []any
	}{
		{`INSERT INTO facilities (name, created_at, updated_at) VALUES ('Default', ?, ?), ('North', ?, ?), ('South', ?, ?), ('Retired', ?, ?)`,
			[]any{now, now, now, now, now, now, now, now}},
		{`UPDATE facilities SET deleted_at = ? WHERE name = 'Retired'`, []any{now}},
		{`INSERT INTO users (id, username) VALUES (1, 'SuperAdmin'), (?, ?)`, []any{residentClaims.UserID, residentClaims.Username}},
	} {
		require.NoError(t, db.Exec(stmt.sql, stmt.args...).Error, stmt.sql)
	}
}

// routeResponse is a route's answer.
type routeResponse struct {
	status int
	header http.Header
	body   string
}

// call sends a request as c, with body as JSON unless it is nil.
func call(t *testing.T, ts *httptest.Server, c *identity.Claims, method, path string, body any) routeResponse {
	t.Helper()
	var reqBody io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		require.NoError(t, err)
		reqBody = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(t.Context(), method, ts.URL+path, reqBody)
	require.NoError(t, err)
	claims, err := json.Marshal(c)
	require.NoError(t, err)
	req.Header.Set(testClaimsHeader, string(claims))
	req.Header.Set("Content-Type", "application/json")
	resp, err := ts.Client().Do(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	b, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return routeResponse{status: resp.StatusCode, header: resp.Header, body: string(b)}
}

// message is the "message" of a JSON answer.
func message(t *testing.T, resp routeResponse) string {
	t.Helper()
	var body struct {
		Message string `json:"message"`
	}
	require.NoError(t, json.Unmarshal([]byte(resp.body), &body), resp.body)
	return body.Message
}

func TestLearningRecordRoutes(t *testing.T) {
	ts, db := newRouteTest(t)

	tooLong := strings.Repeat("a", 121)

	// The cases run in order against one database, so later reads show the
	// earlier writes.
	tests := []struct {
		name   string
		method string
		path   string
		body   any
		status int
	}{
		{"facilities", http.MethodGet, "/api/learning-record/facilities", nil, http.StatusOK},
		{"entries_empty", http.MethodGet, "/api/learning-record/entries", nil, http.StatusOK},
		{"entry_create_both_facility_fields", http.MethodPost, "/api/learning-record/entries",
			map[string]any{"facility_id": 1, "facility_other": "Community Center"}, http.StatusBadRequest},
		{"entry_create_bad_facility_id", http.MethodPost, "/api/learning-record/entries",
			map[string]any{"facility_id": 9999}, http.StatusBadRequest},
		{"entry_create_facility_other_too_long", http.MethodPost, "/api/learning-record/entries",
			map[string]any{"facility_other": tooLong}, http.StatusBadRequest},
		{"entry_create", http.MethodPost, "/api/learning-record/entries",
			map[string]any{"client_id": "entry-1", "program_name": "GED Prep", "facility_id": northID, "completion_date": "2026-01-15", "confidence": "high", "summary": "Finished the program."},
			http.StatusCreated},
		{"entry_create_facility_other", http.MethodPost, "/api/learning-record/entries",
			map[string]any{"client_id": "entry-2", "program_name": "Peer Mentoring", "facility_other": "Community Center", "completion_date": "2026-02-01"},
			http.StatusCreated},
		{"entries_after_creates", http.MethodGet, "/api/learning-record/entries", nil, http.StatusOK},
		{"entry_update", http.MethodPut, "/api/learning-record/entries/1",
			map[string]any{"program_name": "GED Prep (Advanced)", "facility_other": "New Location", "completion_date": "2026-01-20"},
			http.StatusOK},
		{"entry_update_bad_id", http.MethodPut, "/api/learning-record/entries/abc", map[string]any{}, http.StatusBadRequest},
		{"entry_delete", http.MethodDelete, "/api/learning-record/entries/1", nil, http.StatusOK},
		{"entries_after_delete", http.MethodGet, "/api/learning-record/entries", nil, http.StatusOK},
		{"draft_get_none", http.MethodGet, "/api/learning-record/draft", nil, http.StatusOK},
		// draft_upsert (PUT /api/learning-record/draft) is a pre-existing,
		// pinned-as-broken route; see README.md's Tests section.
		{"draft_upsert", http.MethodPut, "/api/learning-record/draft",
			map[string]any{"client_id": "draft-upsert", "program_name": "Welding Basics"}, http.StatusInternalServerError},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := call(t, ts, residentClaims, tt.method, tt.path, tt.body)
			require.Equal(t, tt.status, resp.status, resp.body)
			expectGolden(t, tt.name, resp.body)
		})
	}

	// The GET and DELETE draft routes don't go through UpsertDraft, so a
	// draft is seeded directly to cover them independently of that route.
	draft := models.LearningRecordEntry{UserID: residentClaims.UserID, ClientID: "draft-1", IsDraft: true, ProgramName: "Welding Basics", StepIndex: 1, UiPhase: "survey"}
	require.NoError(t, db.Create(&draft).Error)
	for _, tt := range []struct {
		name   string
		method string
		path   string
		status int
	}{
		{"draft_get_after_seed", http.MethodGet, "/api/learning-record/draft", http.StatusOK},
		{"draft_delete_missing_client_id", http.MethodDelete, "/api/learning-record/draft", http.StatusBadRequest},
		{"draft_delete", http.MethodDelete, "/api/learning-record/draft?client_id=draft-1", http.StatusOK},
		{"draft_get_after_delete", http.MethodGet, "/api/learning-record/draft", http.StatusOK},
	} {
		t.Run(tt.name, func(t *testing.T) {
			resp := call(t, ts, residentClaims, tt.method, tt.path, nil)
			require.Equal(t, tt.status, resp.status, resp.body)
			expectGolden(t, tt.name, resp.body)
		})
	}

	// The HTTP response never shows who recorded the deletion; the audit
	// trail (written with the request's context, which records the actor
	// from its identity claims) does.
	t.Run("entry_delete_records_audit_history", func(t *testing.T) {
		var history struct {
			Action           string
			AdminID          *uint
			AttendanceStatus *string
		}
		err := db.Table("user_account_history").
			Select("action, admin_id, attendance_status").
			Where("user_id = ? AND action = ?", residentClaims.UserID, "learning_record_deleted").
			Order("created_at DESC").Limit(1).Scan(&history).Error
		require.NoError(t, err)
		require.Equal(t, "learning_record_deleted", history.Action)
		require.NotNil(t, history.AdminID)
		require.Equal(t, residentClaims.UserID, *history.AdminID)
		// Core wrote this column's zero value before the move; a struct that
		// drops the field writes NULL instead, changing the account-history
		// response body.
		require.NotNil(t, history.AttendanceStatus)
		require.Equal(t, "", *history.AttendanceStatus)
	})

	// The assertion above reads the row directly; this renders it the way
	// core's GET /api/users/{id}/account-history serves it to an admin, the
	// real consumer of attendance_status, so both boundaries are pinned.
	// That route stays in core, so coreAccountHistory runs its query here.
	t.Run("account_history_endpoint", func(t *testing.T) {
		expectGolden(t, "account_history", coreAccountHistory(t, db, residentClaims.UserID))
	})
}

// TestLearningRecordRouteAccess pins each route's feature flag and access
// level against every route this service serves.
func TestLearningRecordRouteAccess(t *testing.T) {
	ts, _ := newRouteTest(t)

	routes := []struct {
		method, path string
	}{
		{http.MethodGet, "/api/learning-record/facilities"},
		{http.MethodGet, "/api/learning-record/entries"},
		{http.MethodPost, "/api/learning-record/entries"},
		{http.MethodPut, "/api/learning-record/entries/1"},
		{http.MethodDelete, "/api/learning-record/entries/1"},
		{http.MethodGet, "/api/learning-record/draft"},
		{http.MethodPut, "/api/learning-record/draft"},
		{http.MethodDelete, "/api/learning-record/draft"},
	}
	require.Len(t, (&server{}).routes(), len(routes), "every route is listed here")

	// The same resident, with learning_record off at their facility.
	off := withoutLearningRecord(residentClaims)

	for _, rt := range routes {
		t.Run(rt.method+" "+rt.path, func(t *testing.T) {
			// Every route requires the learning_record feature.
			resp := call(t, ts, off, rt.method, rt.path, map[string]any{})
			require.Equal(t, http.StatusUnauthorized, resp.status, resp.body)
			require.Equal(t, "Feature not enabled", message(t, resp))

			// Every route is open to any logged-in user: a resident with the
			// feature gets past the checks. The body isn't valid JSON; a route
			// that reads one (POST/PUT) rejects it before doing anything else.
			// DELETE never reads a body, so it still runs its normal delete.
			resp = call(t, ts, residentClaims, rt.method, rt.path, "not an object")
			require.NotEqual(t, http.StatusUnauthorized, resp.status, resp.body)
		})
	}
}

// Every answer carries core's security headers.
func TestSecurityHeaders(t *testing.T) {
	ts, _ := newRouteTest(t)
	resp := call(t, ts, residentClaims, http.MethodGet, "/api/learning-record/entries", nil)
	require.Equal(t, http.StatusOK, resp.status, resp.body)
	assert.Equal(t, "DENY", resp.header.Get("X-Frame-Options"))
	assert.Equal(t, "nosniff", resp.header.Get("X-Content-Type-Options"))
	assert.Equal(t, "max-age=31536000; includeSubDomains", resp.header.Get("Strict-Transport-Security"))
	assert.Equal(t, "strict-origin-when-cross-origin", resp.header.Get("Referrer-Policy"))
}

// Each request is counted under its route's pattern, as core counted it,
// including the ones a check refuses.
func TestMetrics(t *testing.T) {
	ts, _ := newRouteTest(t)
	const route = "/api/learning-record/entries/{id}"
	bad := testutil.ToFloat64(responseStatus.WithLabelValues(route, http.MethodPut, "400"))
	refused := testutil.ToFloat64(errorCount.WithLabelValues("/api/learning-record/entries", http.MethodGet, "401"))

	for _, id := range []string{"abc", "xyz"} {
		resp := call(t, ts, residentClaims, http.MethodPut, "/api/learning-record/entries/"+id, map[string]any{})
		require.Equal(t, http.StatusBadRequest, resp.status, resp.body)
	}
	resp := call(t, ts, withoutLearningRecord(residentClaims), http.MethodGet, "/api/learning-record/entries", nil)
	require.Equal(t, http.StatusUnauthorized, resp.status, resp.body)

	assert.Equal(t, bad+2, testutil.ToFloat64(responseStatus.WithLabelValues(route, http.MethodPut, "400")), "one series for every entry")
	assert.Equal(t, refused+1, testutil.ToFloat64(errorCount.WithLabelValues("/api/learning-record/entries", http.MethodGet, "401")))
}

// The routes are behind identity.SessionChecker: without a login session,
// NewHandler answers its 401 before any route runs. /api/metrics needs no
// login.
func TestNewHandlerRequiresLogin(t *testing.T) {
	sessions := identity.NewSessionChecker("http://kratos.invalid", "http://core.invalid", nil)
	h := NewHandler(newTestDB(t), sessions.Require)

	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/learning-record/entries", nil))
	require.Equal(t, http.StatusUnauthorized, w.Code)
	require.JSONEq(t, `{"message":"not logged in","data":null}`, w.Body.String())

	w = httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/metrics", nil))
	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Body.String(), "http_requests_total")
}

// coreAccountHistory is core's answer to GET /api/users/{id}/account-history
// for userID's first page: core's GetUserAccountHistory query, with no
// category filter, in core's paginated JSON. It is here only to show the row
// DeleteEntry adds still reads the same in core.
func coreAccountHistory(t *testing.T, db *database.DB, userID uint) string {
	t.Helper()
	type activityHistoryResponse struct {
		Action                  string     `json:"action"`
		CreatedAt               *time.Time `json:"created_at"`
		FieldName               *string    `json:"field_name"`
		NewValue                *string    `json:"new_value"`
		OldValue                *string    `json:"old_value"`
		UserID                  *uint      `json:"user_id"`
		UserUsername            *string    `json:"user_username"`
		AdminUsername           *string    `json:"admin_username"`
		FacilityName            *string    `json:"facility_name"`
		ProgramClassesHistoryID *uint      `json:"program_classes_history_id"`
		AttendanceStatus        *string    `json:"attendance_status"`
		ClassName               *string    `json:"class_name"`
		SessionDate             *time.Time `json:"session_date"`
		ClassHistoryID          *uint      `json:"-"`
	}
	const page, perPage = 1, 20
	history := make([]activityHistoryResponse, 0, perPage)
	tx := db.Table("user_account_history uah").
		Select(`uah.action, uah.created_at, uah.user_id,
				users.username AS user_username,
				admins.username AS admin_username,
				facilities.name AS facility_name,
				uah.attendance_status, uah.class_name, uah.session_date,
				uah.program_classes_history_id AS class_history_id`).
		Joins("INNER JOIN users ON uah.user_id = users.id").
		Joins("LEFT JOIN users admins ON uah.admin_id = admins.id").
		Joins("LEFT JOIN facilities ON uah.facility_id = facilities.id").
		Where("uah.user_id = ?", userID)
	var total int64
	require.NoError(t, tx.Count(&total).Error)
	require.NoError(t, tx.Order("uah.created_at desc").Offset((page-1)*perPage).Limit(perPage).Scan(&history).Error)
	lastPage := int(total) / perPage
	if int(total)%perPage > 0 {
		lastPage++
	}
	b, err := json.Marshal(map[string]any{
		"message": "",
		"data":    history,
		"meta":    map[string]any{"current_page": page, "last_page": lastPage, "per_page": perPage, "total": total},
	})
	require.NoError(t, err)
	return string(b)
}

// timeKey matches the JSON fields that hold a time that changes every run.
var timeKey = regexp.MustCompile(`(_at|_ts)$`)

// expectGolden compares body with testdata/learning_record/<name>.golden. A
// JSON body is compared after indenting it and replacing its times with
// "<time>".
func expectGolden(t *testing.T, name, body string) {
	t.Helper()
	got := normalizeBody(t, body)
	path := filepath.Join("testdata", "learning_record", name+".golden")
	if *updateGolden {
		require.NoError(t, os.WriteFile(path, []byte(got), 0o644))
		return
	}
	want, err := os.ReadFile(path)
	require.NoError(t, err, "missing golden file; run with -update to create it")
	require.Equal(t, string(want), got)
}

func normalizeBody(t *testing.T, body string) string {
	t.Helper()
	var v any
	if err := json.Unmarshal([]byte(body), &v); err != nil {
		return body
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	require.NoError(t, enc.Encode(maskTimes(v)))
	return buf.String()
}

// maskTimes replaces every time with "<time>", except a zero time, which is
// kept because it shows a field the route never set.
func maskTimes(v any) any {
	switch v := v.(type) {
	case map[string]any:
		for k, x := range v {
			if s, ok := x.(string); ok && timeKey.MatchString(k) && !strings.HasPrefix(s, "0001-01-01") {
				v[k] = "<time>"
			} else {
				v[k] = maskTimes(x)
			}
		}
	case []any:
		for i, x := range v {
			v[i] = maskTimes(x)
		}
	}
	return v
}

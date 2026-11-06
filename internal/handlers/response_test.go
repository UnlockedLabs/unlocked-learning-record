package handlers

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/UnlockedLabs/unlocked-contracts/identity"
	"github.com/UnlockedLabs/unlocked-contracts/users"
	"github.com/sirupsen/logrus"
	logtest "github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A failed request's error line carries the route's fields, and an admin's
// write also logs core's admin audit line, whose fields include the ones the
// handler added, as core's did for Learning Record's routes. Anyone else's
// request logs no audit line.
func TestHandleLogs(t *testing.T) {
	oldHooks := logrus.StandardLogger().ReplaceHooks(make(logrus.LevelHooks))
	defer logrus.StandardLogger().ReplaceHooks(oldHooks)
	hook := logtest.NewGlobal()

	failing := handle(func(_ http.ResponseWriter, _ *http.Request, log sLog) error {
		log.add("entry_id", 1)
		return errors.New("boom")
	})
	request := func(method string, c *identity.Claims) *httptest.ResponseRecorder {
		r := httptest.NewRequestWithContext(identity.NewContext(t.Context(), c), method, "/api/learning-record/entries/1", nil)
		r.Header.Set("X-Forwarded-For", "10.0.0.9, 10.0.0.1")
		w := httptest.NewRecorder()
		failing.ServeHTTP(w, r)
		return w
	}
	facAdmin := &identity.Claims{UserID: 3, Username: "facadmin", Role: users.FacilityAdmin, FacilityID: 2, FacilityName: "North", SessionID: "s1"}

	t.Run("admin write", func(t *testing.T) {
		hook.Reset()
		w := request(http.MethodPut, facAdmin)
		require.Equal(t, http.StatusInternalServerError, w.Code)
		entries := hook.AllEntries()
		require.Len(t, entries, 2)

		errLine, audit := entries[0], entries[1]
		assert.Equal(t, logrus.ErrorLevel, errLine.Level)
		assert.Equal(t, "learningrecord", errLine.Data["module"])
		assert.Equal(t, http.MethodPut, errLine.Data["method"])
		assert.Equal(t, "/api/learning-record/entries/1", errLine.Data["path"])
		assert.Equal(t, uint(3), errLine.Data["admin_id"])
		assert.Equal(t, "facadmin", errLine.Data["username"])

		assert.Equal(t, logrus.Fields{
			"module":        "learningrecord",
			"method":        http.MethodPut,
			"path":          "/api/learning-record/entries/1",
			"admin_id":      uint(3),
			"username":      "facadmin",
			"role":          users.FacilityAdmin,
			"session_id":    "s1",
			"facility_id":   uint(2),
			"facility_name": "North",
			"ip_address":    "10.0.0.9",
			"entry_id":      1,
			"audit":         "admin_action",
		}, audit.Data)
	})

	t.Run("admin read", func(t *testing.T) {
		hook.Reset()
		request(http.MethodGet, facAdmin)
		require.Len(t, hook.AllEntries(), 1)
		assert.NotContains(t, hook.LastEntry().Data, "audit")
	})

	t.Run("resident write", func(t *testing.T) {
		hook.Reset()
		w := request(http.MethodPut, residentClaims)
		require.Equal(t, http.StatusInternalServerError, w.Code)
		require.Len(t, hook.AllEntries(), 1)
		assert.Equal(t, logrus.ErrorLevel, hook.LastEntry().Level)
		assert.Equal(t, 1, hook.LastEntry().Data["entry_id"])
		assert.NotContains(t, hook.LastEntry().Data, "audit")
		assert.NotContains(t, hook.LastEntry().Data, "admin_id")
	})
}

// Package handlers is Learning Record's HTTP API: a resident's learning
// record entries and the in-progress draft of one. It was a module in core
// until EN-208.
package handlers

import (
	"context"
	"net/http"

	"github.com/UnlockedLabs/unlocked-contracts/features"
	"github.com/UnlockedLabs/unlocked-learning-record/internal/database"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// server is Learning Record's routes and what they use.
type server struct {
	db *database.DB
}

// featureChecker says whether a feature is on at a facility, as
// features.Checker does by asking core. Its bool is always safe to act on; an
// error only says core couldn't answer just now.
type featureChecker interface {
	Enabled(ctx context.Context, facilityID uint, f features.Access) (bool, error)
}

// NewHandler serves Learning Record's routes, each behind requireLogin and
// its feature check, and its metrics on /api/metrics for Prometheus.
// requireLogin is identity.SessionChecker's Require; flags is a
// features.Checker.
func NewHandler(db *database.DB, requireLogin func(http.Handler) http.Handler, flags featureChecker) http.Handler {
	srv := &server{db: db}
	mux := http.NewServeMux()
	mux.Handle("/api/learning-record/", newRouter(srv.routes(), requireLogin, flags))
	mux.Handle("GET /api/metrics", promhttp.Handler())
	return mux
}

// Package handlers is Learning Record's HTTP API: a resident's learning
// record entries and the in-progress draft of one. It was a module in core
// until EN-208.
package handlers

import (
	"net/http"

	"github.com/UnlockedLabs/unlocked-learning-record/internal/database"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// server is Learning Record's routes and what they use.
type server struct {
	db *database.DB
}

// NewHandler serves Learning Record's routes, each behind requireLogin and
// its feature check, and its metrics on /api/metrics for Prometheus.
// requireLogin is identity.SessionChecker's Require.
func NewHandler(db *database.DB, requireLogin func(http.Handler) http.Handler) http.Handler {
	srv := &server{db: db}
	mux := http.NewServeMux()
	mux.Handle("/api/learning-record/", newRouter(srv.routes(), requireLogin))
	mux.Handle("GET /api/metrics", promhttp.Handler())
	return mux
}

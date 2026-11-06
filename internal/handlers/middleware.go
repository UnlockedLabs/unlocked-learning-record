package handlers

import (
	"net/http"
	"os"

	"github.com/UnlockedLabs/unlocked-contracts/features"
	"github.com/sirupsen/logrus"
)

// The router puts each route behind the same checks core ran on it before
// the move (EN-208), in the same order and with the same answers: the login
// session and CSRF token, then the route's features. Every Learning Record
// route is open to any logged-in user, so there is no admin check.

// route is one HTTP route.
type route struct {
	Pattern string
	Handler http.Handler
	// Features turns the route off unless every listed feature is on at the
	// caller's facility.
	Features []features.Access
}

// newRouter serves routes, each behind requireLogin and its feature check,
// and records each one's metrics. requireLogin checks the session and puts
// the caller's identity.Claims in the request's context, as
// identity.SessionChecker's Require does.
func newRouter(routes []route, requireLogin func(http.Handler) http.Handler) http.Handler {
	mux := http.NewServeMux()
	for _, rt := range routes {
		mux.Handle(rt.Pattern, observe(rt.Pattern, requireLogin(requireFeatures(rt.Handler, rt.Features))))
	}
	return securityHeaders(mux)
}

// requireFeatures answers core's 401 "Feature not enabled" unless the caller's
// claims have every one of required, as core's checkFeatureAccessMiddleware
// does. The claims come from core with the session, so a feature is on or off
// here exactly when it is in core.
func requireFeatures(next http.Handler, required []features.Access) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims := claimsFrom(r)
		if !claims.HasFeature(required...) {
			logrus.WithFields(logrus.Fields{
				"handler":           "checkFeatureAccessMiddleware",
				"method":            r.Method,
				"path":              r.URL.Path,
				"required_features": required,
				"user_id":           claims.UserID,
				"facility_id":       claims.FacilityID,
				"feature_access":    claims.FeatureAccess,
			}).Warn("feature not enabled for user")
			errorResponse(w, http.StatusUnauthorized, "Feature not enabled")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// securityHeaders sets core's security headers.
func securityHeaders(next http.Handler) http.Handler {
	dev := os.Getenv("APP_ENV") == "dev"
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		if !dev {
			w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		}
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		next.ServeHTTP(w, r)
	})
}

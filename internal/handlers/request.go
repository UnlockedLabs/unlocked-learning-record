package handlers

import (
	"net/http"

	"github.com/UnlockedLabs/unlocked-contracts/identity"
)

// claimsFrom is the caller of a request. The router serves only requests the
// session check has passed, so the claims are always there.
func claimsFrom(r *http.Request) *identity.Claims {
	c, _ := identity.FromContext(r.Context())
	return c
}

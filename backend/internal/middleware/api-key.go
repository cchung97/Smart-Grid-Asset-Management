package middleware

import (
	"crypto/subtle"
	"net/http"

	"github.com/gin-gonic/gin"

	"smart-grid-asset-management/backend/internal/defined"
)

// RequireAPIKey guards the /api endpoints with the shared API key, sent as the
// x-api-key header only (never a query parameter or cookie: those end up in logs,
// history and cross-site requests). It fails closed: with no key configured the
// endpoints are disabled rather than open. Wrong or missing key is 401; the
// comparison is constant-time.
//
// This is a shared secret, not user authentication: anyone who knows the key has
// full access, and nothing records who they are beyond the client IP in the log.
func RequireAPIKey(want string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if want == "" {
			abort(c, http.StatusForbidden, "the API is disabled because no API key is configured on the server")
			return
		}
		if !keyMatches(c.GetHeader(defined.APIKeyHeader), want) {
			abort(c, http.StatusUnauthorized, "missing or invalid API key")
			return
		}
		c.Next()
	}
}

// keyMatches compares in constant time so the check leaks nothing about
// how much of a guessed key was right.
func keyMatches(got, want string) bool {
	return subtle.ConstantTimeCompare([]byte(got), []byte(want)) == 1
}

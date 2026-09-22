package middleware

import (
	"net/http"
	"slices"

	"github.com/gin-gonic/gin"
)

// CORS allows requests from allowedOrigins; an empty list allows every
// origin via a wildcard. This app has no cookies/credentials anywhere
// (see docs/ARCHITECTURE.md's "no auth" scope), so a wildcard default
// carries none of the risk it would for a credentialed API.
func CORS(allowedOrigins []string) gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")

		switch {
		case len(allowedOrigins) == 0:
			c.Header("Access-Control-Allow-Origin", "*")
		case origin == "":
			// same-origin request, no CORS headers needed
		case slices.Contains(allowedOrigins, origin):
			c.Header("Access-Control-Allow-Origin", origin)
			c.Header("Vary", "Origin")
		default:
			abort(c, http.StatusForbidden, "origin not allowed")
			return
		}

		c.Header("Access-Control-Allow-Headers", "Content-Type, Accept, x-api-key")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		c.Header("Access-Control-Max-Age", "600")

		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

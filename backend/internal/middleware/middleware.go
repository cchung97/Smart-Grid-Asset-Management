// Package middleware holds gin middleware: cross-cutting ones applied to
// every request (trace ID, access log, panic recovery, security headers,
// CORS, rate limiting, request size limits) and route-specific guards (the
// API key on /api). Each is a gin.HandlerFunc — or returns one — so it is
// registered with Engine.Use or on a route.
package middleware

import (
	"github.com/gin-gonic/gin"

	"smart-grid-asset-management/backend/internal/entity/dto"
)

// abort ends the request with a plain {"error": message} JSON body.
func abort(c *gin.Context, status int, message string) {
	c.AbortWithStatusJSON(status, dto.ErrorResponse{Error: message})
}

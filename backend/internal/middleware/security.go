package middleware

import "github.com/gin-gonic/gin"

// SecurityHeaders sets a standard set of defensive response headers,
// adapted from the reference project's middleware for this app's actual
// shape: same-origin SPA + API, no cookies/auth, HTTP in Docker Compose
// (a TLS-terminating proxy in front, if any, would still make HSTS
// meaningful — it's inert and harmless over plain HTTP).
func SecurityHeaders(c *gin.Context) {
	h := c.Writer.Header()
	h.Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
	h.Set("X-Content-Type-Options", "nosniff")
	// connect-src/img-src/etc. scoped to 'self' — unlike the reference
	// project, this app has no third-party origins to allow.
	h.Set("Content-Security-Policy", "default-src 'self'; connect-src 'self'; img-src 'self' data:; script-src 'self'; style-src 'self' 'unsafe-inline'; frame-ancestors 'none'; base-uri 'self'; form-action 'self';")
	h.Set("X-Frame-Options", "DENY")
	h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
	h.Set("Permissions-Policy", "geolocation=(), microphone=(), camera=(), payment=(), usb=()")
	h.Set("Cross-Origin-Opener-Policy", "same-origin")
	h.Set("Cross-Origin-Resource-Policy", "same-origin")
	h.Set("X-Permitted-Cross-Domain-Policies", "none")
	c.Next()
}

// docsCSP is the policy for the Swagger UI page only. Its generated index
// runs an inline initialisation script, which the strict global policy
// (script-src 'self') would block, leaving a blank page. Nothing else in
// the app is served under this relaxed policy.
const docsCSP = "default-src 'self'; connect-src 'self'; img-src 'self' data:; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline'; frame-ancestors 'none'; base-uri 'self'; form-action 'self';"

// DocsCSP replaces the strict Content-Security-Policy set by
// SecurityHeaders with docsCSP for the routes it guards.
func DocsCSP(c *gin.Context) {
	c.Header("Content-Security-Policy", docsCSP)
	c.Next()
}

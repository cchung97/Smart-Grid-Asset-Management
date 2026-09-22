package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"smart-grid-asset-management/backend/internal/defined"
)

func init() { gin.SetMode(gin.TestMode) }

func TestDocsCSP_OverridesTheStrictPolicyOnlyForItsHandler(t *testing.T) {
	noop := func(c *gin.Context) { c.Status(http.StatusNoContent) }
	e := gin.New()
	e.Use(SecurityHeaders)
	e.GET("/api/x", noop)
	e.GET("/swagger/", DocsCSP, noop)

	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/x", nil))
	if csp := rec.Header().Get("Content-Security-Policy"); !strings.Contains(csp, "script-src 'self';") {
		t.Errorf("API responses must keep script-src 'self' only, got %q", csp)
	}

	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/swagger/", nil))
	if csp := rec.Header().Get("Content-Security-Policy"); !strings.Contains(csp, "script-src 'self' 'unsafe-inline'") {
		t.Errorf("swagger page policy = %q, want inline scripts allowed", csp)
	}
}

func TestRequireAPIKey(t *testing.T) {
	tests := []struct {
		name   string
		want   string // configured key
		header string
		query  string
		status int
	}{
		{"right key", "s3cret", "s3cret", "", http.StatusNoContent},
		{"wrong key", "s3cret", "nope", "", http.StatusUnauthorized},
		{"no key sent", "s3cret", "", "", http.StatusUnauthorized},
		{"key in the query string is not accepted", "s3cret", "", "?api_key=s3cret", http.StatusUnauthorized},
		{"prefix of the key is not enough", "s3cret", "s3cre", "", http.StatusUnauthorized},
		{"no key configured: fails closed", "", "", "", http.StatusForbidden},
		{"no key configured, empty header still refused", "", "anything", "", http.StatusForbidden},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := gin.New()
			e.DELETE("/x", RequireAPIKey(tt.want), func(c *gin.Context) { c.Status(http.StatusNoContent) })
			req := httptest.NewRequest(http.MethodDelete, "/x"+tt.query, nil)
			if tt.header != "" {
				req.Header.Set(defined.APIKeyHeader, tt.header)
			}
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)
			if rec.Code != tt.status {
				t.Fatalf("status = %d, want %d; body %s", rec.Code, tt.status, rec.Body.String())
			}
			if tt.status != http.StatusNoContent && !strings.Contains(rec.Body.String(), `"error"`) {
				t.Errorf("body %q is not an ErrorResponse", rec.Body.String())
			}
		})
	}
}

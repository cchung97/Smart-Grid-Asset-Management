package middleware

import (
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"

	"github.com/gin-gonic/gin"

	"smart-grid-asset-management/backend/internal/base/logs"
	"smart-grid-asset-management/backend/internal/defined"
)

func TestTraceID(t *testing.T) {
	tests := []struct {
		name        string
		headerValue string
		wantEchoed  string // empty means "any non-empty generated value"
	}{
		{
			name:        "generates an ID when the header is absent",
			headerValue: "",
		},
		{
			name:        "reuses a client-supplied ID",
			headerValue: "abc123",
			wantEchoed:  "abc123",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotCtxID string
			e := gin.New()
			e.Use(TraceID)
			e.GET("/healthz", func(c *gin.Context) { gotCtxID = logs.TraceIDFromContext(c.Request.Context()) })

			req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
			if tt.headerValue != "" {
				req.Header.Set(defined.TraceIDHeader, tt.headerValue)
			}
			rec := httptest.NewRecorder()

			e.ServeHTTP(rec, req)

			respID := rec.Header().Get(defined.TraceIDHeader)
			if respID == "" {
				t.Fatal("expected a non-empty X-Trace-Id response header")
			}
			if gotCtxID != respID {
				t.Fatalf("context trace ID %q does not match response header %q", gotCtxID, respID)
			}
			if tt.wantEchoed == "" && !regexp.MustCompile(`^[0-9a-f]{8}$`).MatchString(respID) {
				t.Fatalf("generated trace ID %q, want 8 lowercase hex characters", respID)
			}
			if tt.wantEchoed != "" && respID != tt.wantEchoed {
				t.Fatalf("expected the supplied trace ID %q to be echoed unchanged, got %q", tt.wantEchoed, respID)
			}
		})
	}
}

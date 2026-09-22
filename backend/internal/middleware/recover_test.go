package middleware

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"smart-grid-asset-management/backend/internal/base/logs"
	"smart-grid-asset-management/backend/internal/defined"
)

func TestRecover_TurnsPanicInto500WithTraceID(t *testing.T) {
	var buf bytes.Buffer
	logs.SetOutput(&buf)
	t.Cleanup(func() { logs.SetOutput(os.Stdout) })

	h := gin.New()
	h.Use(TraceID, AccessLog, Recover)
	h.GET("/x", func(*gin.Context) { panic("boom") })

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.Header.Set(defined.TraceIDHeader, "trace-123")
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("body is not JSON: %v (%q)", err, rec.Body.String())
	}
	if body["trace_id"] != "trace-123" {
		t.Errorf("trace_id = %q, want trace-123", body["trace_id"])
	}
	if strings.Contains(rec.Body.String(), "boom") {
		t.Errorf("response leaks the panic value: %q", rec.Body.String())
	}
	out := buf.String()
	for _, want := range []string{"panic in handler", "panic=boom", "tid=trace-123", "status=500"} {
		if !strings.Contains(out, want) {
			t.Errorf("log output missing %q:\n%s", want, out)
		}
	}
}

func TestAccessLog_RecordsStatusAndPath(t *testing.T) {
	var buf bytes.Buffer
	logs.SetOutput(&buf)
	t.Cleanup(func() { logs.SetOutput(os.Stdout) })

	h := gin.New()
	h.Use(TraceID, AccessLog)
	h.GET("/nope", func(c *gin.Context) { c.Status(http.StatusNotFound) })
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/nope", nil))

	out := buf.String()
	for _, want := range []string{"msg=request", "path=/nope", "status=404", "method=GET"} {
		if !strings.Contains(out, want) {
			t.Errorf("log output missing %q:\n%s", want, out)
		}
	}
}

package middleware

import (
	"net/http"
	"runtime/debug"

	"github.com/gin-gonic/gin"

	"smart-grid-asset-management/backend/internal/base/logs"
	"smart-grid-asset-management/backend/internal/entity/dto"
)

// Recover turns a panic in a handler into a logged 500 instead of a
// dropped connection. It keeps the stack in the same trace-correlated log
// stream as everything else and gives the client a well-formed JSON error
// carrying only the trace ID, never the panic value. Register it after
// TraceID (so the log line carries the trace ID) and inside AccessLog (so
// the 500 is access-logged).
func Recover(c *gin.Context) {
	defer func() {
		rec := recover()
		if rec == nil {
			return
		}
		if rec == http.ErrAbortHandler {
			panic(rec) // net/http's own abort signal; must propagate
		}
		ctx := c.Request.Context()
		logs.WithCtx(ctx).Error("panic in handler",
			"method", c.Request.Method, "path", c.Request.URL.Path,
			"panic", rec, "stack", string(debug.Stack()))
		c.AbortWithStatusJSON(http.StatusInternalServerError, dto.ErrorResponse{
			Error:   "internal server error",
			TraceID: logs.TraceIDFromContext(ctx),
		})
	}()
	c.Next()
}

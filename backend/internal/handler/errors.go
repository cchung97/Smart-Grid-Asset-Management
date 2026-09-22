package handler

import (
	"context"
	"encoding/csv"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"smart-grid-asset-management/backend/internal/base/csvx"
	"smart-grid-asset-management/backend/internal/base/errx"
	"smart-grid-asset-management/backend/internal/base/logs"
	"smart-grid-asset-management/backend/internal/entity/dto"
	"smart-grid-asset-management/backend/internal/middleware"
	"smart-grid-asset-management/backend/internal/service"
)

// writeError maps an error from csvx or the service layer to an HTTP
// response on c and logs it.
//
// What is safe to show a client — a bad extension, a missing column, a
// malformed line — is returned verbatim, because it tells the user how to
// fix their request. Everything unexpected becomes a generic 500 carrying
// only the trace ID; the real error goes to the log with the same ID.
func writeError(c *gin.Context, err error) {
	var (
		csvErr      *csvx.Error
		parseErr    *csv.ParseError
		schemaErr   *service.SchemaError
		inputErr    *errx.InputError
		conflictErr *errx.ConflictError
	)
	r := c.Request
	body := dto.ErrorResponse{}
	var status int

	switch {
	case middleware.IsBodyTooLarge(err):
		status, body.Error = http.StatusRequestEntityTooLarge, "request body too large"
	case errors.As(err, &csvErr):
		status, body.Error = csvStatus(csvErr.Code), csvErr.Msg
		if errors.As(err, &parseErr) {
			line := parseErr.StartLine
			body.Line = &line
		}
	case errors.As(err, &schemaErr):
		status, body.Error = http.StatusUnprocessableEntity, schemaErr.Error()
	case errors.Is(err, service.ErrNoDataRows):
		status, body.Error = http.StatusUnprocessableEntity, err.Error()
	case errors.As(err, &inputErr):
		status, body.Error = http.StatusBadRequest, inputErr.Msg
	case errors.As(err, &conflictErr):
		status, body.Error = http.StatusConflict, conflictErr.Msg
	case errors.Is(err, service.ErrPreviewStale):
		status, body.Error = http.StatusPreconditionFailed, err.Error()
	case errors.Is(err, errx.ErrNotFound):
		status, body.Error = http.StatusNotFound, "not found"
	case errors.Is(err, errx.ErrConflict):
		status, body.Error = http.StatusConflict, "the stored data changed while the request was being processed; please retry"
	case errors.Is(err, context.DeadlineExceeded):
		status, body.Error = http.StatusServiceUnavailable, "the request took too long and was cancelled"
	case errors.Is(err, context.Canceled):
		status, body.Error = http.StatusBadRequest, "request cancelled"
	default:
		status, body.Error = http.StatusInternalServerError, "internal server error"
	}
	body.TraceID = logs.TraceIDFromContext(r.Context())

	log := logs.WithCtx(r.Context())
	args := []any{"method", r.Method, "path", r.URL.Path, "status", status, "err", err}
	if body.Line != nil {
		args = append(args, "line", *body.Line)
	}
	if status >= http.StatusInternalServerError {
		log.Error("request failed", args...)
	} else {
		log.Warn("request rejected", args...)
	}
	c.JSON(status, body)
}

// csvStatus maps a structural CSV problem to its status code.
func csvStatus(code csvx.ErrorCode) int {
	switch code {
	case csvx.ErrCodeInvalidExtension:
		return http.StatusUnsupportedMediaType
	case csvx.ErrCodeMalformedCSV:
		return http.StatusUnprocessableEntity
	default: // missing_file, empty_file
		return http.StatusBadRequest
	}
}

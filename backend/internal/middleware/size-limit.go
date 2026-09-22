package middleware

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
)

// RequestSizeLimit rejects a request whose declared Content-Length
// exceeds maxBytes, and caps any body without a declared length (e.g.
// chunked transfer) via http.MaxBytesReader.
//
// The cap surfaces as a read error once a handler reads past it, so a
// handler consuming a body wrapped by this middleware (e.g. the CSV upload
// handler) must itself check IsBodyTooLarge(err) and answer 413.
func RequestSizeLimit(maxBytes int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.ContentLength > maxBytes {
			abort(c, http.StatusRequestEntityTooLarge, "request body too large")
			return
		}
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxBytes)
		c.Next()
	}
}

// IsBodyTooLarge reports whether err came from a RequestSizeLimit-wrapped
// body exceeding its cap.
func IsBodyTooLarge(err error) bool {
	var mbe *http.MaxBytesError
	return errors.As(err, &mbe)
}

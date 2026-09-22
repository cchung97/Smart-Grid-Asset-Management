package dto

// ErrorResponse is the body of every non-2xx response that has no richer
// shape of its own (structural CSV problems, 404s, bad parameters, 5xx).
type ErrorResponse struct {
	// Error is a human-readable, user-safe message. For 5xx it is generic —
	// internals are only ever written to the server log.
	Error string `json:"error" example:"file \"assets.txt\" must have a .csv extension" validate:"required"`
	// Line is the 1-indexed source line of a structurally malformed CSV, when known.
	Line *int `json:"line,omitempty" example:"42"`
	// TraceID matches the X-Trace-Id response header and the server log lines
	// for this request; quote it when reporting a 5xx.
	TraceID string `json:"trace_id,omitempty" example:"7f6c1a52-3c1e-4a51-9d0b-5b1b2f7a9c11"`
}

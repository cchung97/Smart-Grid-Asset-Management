package defined

// Names that are part of the HTTP contract with clients.
const (
	// UploadField is the multipart form field the CSV must be sent under.
	UploadField = "file"
	// FingerprintField is the optional multipart form field that carries the
	// fingerprint a preview returned, so the commit can refuse if the outcome
	// changed since the user reviewed it.
	FingerprintField = "expected_fingerprint"

	// TraceIDHeader is read from the incoming request (if a caller already
	// has a trace ID to propagate) and always echoed back on the response.
	TraceIDHeader = "X-Trace-Id"

	// APIKeyHeader carries the shared API key that the delete endpoint requires.
	APIKeyHeader = "x-api-key"
)

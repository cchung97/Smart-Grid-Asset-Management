// Package defined is the one place the app's shared constants live: the
// limits, wire formats and protocol names that more than one package (or a
// reader of the API docs) needs to agree on.
//
// It is a leaf — it imports nothing from this module — so any layer may
// depend on it. Constants that are private tuning of a single algorithm
// (batch sizes, recursion depth, a local enum) stay next to the code that
// uses them, and nothing under base/ may import this package.
package defined

import "time"

// Search limits: q is bounded so a request cannot ask the trigram index to
// chew on an arbitrarily long pattern, and results are capped.
const (
	MaxSearchQueryLen = 200
	DefaultSearchSize = 50
	MaxSearchSize     = 200
)

// List limits for GET /api/assets and GET /api/imports.
const (
	DefaultListSize       = 25
	MaxListSize           = 200
	DefaultImportListSize = 25
	MaxImportListSize     = 100
)

// Import limits.
const (
	// MaxLoggedRejections bounds per-row log lines for one import, so a file
	// of 100k bad rows cannot flood the log. The response still lists them all.
	MaxLoggedRejections = 50
	// MaxFilenameRunes bounds the stored/logged upload filename.
	MaxFilenameRunes = 255
	// ImportTimeout bounds one whole import (parse, validate, commit).
	ImportTimeout = 2 * time.Minute
)

// Request size limits.
const (
	// MaxUploadMemory caps how much of a multipart upload is buffered in
	// memory; anything beyond this spills to a temp file.
	MaxUploadMemory = 32 << 20
	// DefaultMaxRequestBytes caps a request body when MAX_REQUEST_BYTES is unset.
	DefaultMaxRequestBytes int64 = 10 << 20 // 10 MiB
)

// Rate-limit defaults, used when the limiter is built with zero values.
const (
	DefaultRateLimit  uint          = 100
	DefaultRateWindow time.Duration = 10 * time.Second
)

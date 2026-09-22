package dto

// ImportResponse is the response body for POST /api/imports and
// GET /api/imports/{id} (see ARCHITECTURE.md section 3). A commit stores the
// rows that passed validation and skips the rest: Committed is true when at
// least one row was stored, ImportedRows + RejectedRows = TotalRows, and
// Rejections lists every skipped row.
type ImportResponse struct {
	ImportID     string      `json:"import_id" format:"uuid" example:"0b5f3c7e-8a52-4a55-9b0e-0c9f8a3f6d11" validate:"required"`
	TotalRows    int         `json:"total_rows" example:"222" validate:"required"`
	ImportedRows int         `json:"imported_rows" example:"0" validate:"required"`
	RejectedRows int         `json:"rejected_rows" example:"8" validate:"required"`
	Committed    bool        `json:"committed" example:"false" validate:"required"`
	Rejections   []Rejection `json:"rejections" validate:"required"`
	// IgnoredColumns lists header columns the importer does not recognise;
	// they are skipped (not an error) and not stored.
	IgnoredColumns []string `json:"ignored_columns" example:"legacy_ref,notes" validate:"required"`
}

// Rejection describes a single CSV row that failed validation.
type Rejection struct {
	// CSVRow is the row's original 1-indexed line in the uploaded file
	// (the header is line 1), not its position after any reordering.
	CSVRow  int    `json:"csv_row" example:"178" validate:"required"`
	AssetID string `json:"asset_id" example:"TX-NR" validate:"required"`
	// Reason holds every problem found on the row, joined with "; ".
	Reason string `json:"reason" example:"rating_kva cannot be negative (-500)" validate:"required"`
}

// ImportPreviewResponse is the outcome of validating a file without storing
// anything (POST /api/imports/preview). Fingerprint identifies exactly this
// file and this outcome: send it back as expected_fingerprint when confirming
// and the commit is refused (412) if the file or the stored data changed since.
type ImportPreviewResponse struct {
	TotalRows      int         `json:"total_rows" example:"222" validate:"required"`
	ImportableRows int         `json:"importable_rows" example:"206" validate:"required"`
	RejectedRows   int         `json:"rejected_rows" example:"16" validate:"required"`
	Rejections     []Rejection `json:"rejections" validate:"required"`
	// IgnoredColumns lists header columns the importer does not recognise.
	IgnoredColumns []string `json:"ignored_columns" example:"legacy_ref,notes" validate:"required"`
	// Fingerprint is a SHA-256 hex digest of the file's cells and of the
	// validation outcome.
	Fingerprint string `json:"fingerprint" example:"9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08" validate:"required"`
}

// ImportSummary is one past import in the activity list.
type ImportSummary struct {
	ImportID     string `json:"import_id" format:"uuid" example:"0b5f3c7e-8a52-4a55-9b0e-0c9f8a3f6d11" validate:"required"`
	Filename     string `json:"filename" example:"grid_assets.csv" validate:"required"`
	ClientIP     string `json:"client_ip" example:"203.0.113.7" validate:"required"`
	TotalRows    int    `json:"total_rows" example:"222" validate:"required"`
	ImportedRows int    `json:"imported_rows" example:"206" validate:"required"`
	RejectedRows int    `json:"rejected_rows" example:"16" validate:"required"`
	Committed    bool   `json:"committed" example:"true" validate:"required"`
	CreatedAt    string `json:"created_at" format:"date-time" example:"2026-09-20T08:15:00Z" validate:"required"`
}

// ImportListResponse is one page of past imports, newest first.
type ImportListResponse struct {
	Total  int             `json:"total" example:"3" validate:"required"`
	Limit  int             `json:"limit" example:"25" validate:"required"`
	Offset int             `json:"offset" example:"0" validate:"required"`
	Items  []ImportSummary `json:"items" validate:"required"`
}

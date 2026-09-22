// Package csvx provides structural helpers for turning an uploaded CSV
// file into rows a caller can validate: pulling the file out of a
// multipart HTTP request, checking it is a plausible CSV (right
// extension, non-empty, decodable, not ragged), and parsing it while
// preserving each row's true 1-indexed source line number (see
// docs/ARCHITECTURE.md section 3's csv_row requirement), plus two tiny
// text helpers: blank-row detection and header-name folding. It has no
// knowledge of what the columns mean — asset_id, asset_type, and every
// other domain/hierarchy rule belong to the future import service, not
// here (see CLAUDE.md's "validation never touches the DB" scoping).
package csvx

import (
	"bytes"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

var utf8BOM = []byte("\xef\xbb\xbf")

// byteOrderMark is U+FEFF as a string, for a BOM that survives into a
// header cell (Parse strips the one at the start of the file).
const byteOrderMark = "\uFEFF"

// Row is one CSV data row (the header is returned separately), tagged
// with the 1-indexed line it started on in the original file.
type Row struct {
	Line   int
	Fields []string
}

// IsBlank reports whether every field is empty or whitespace — the
// trailing ",,,," rows spreadsheet exports like to append.
func (r Row) IsBlank() bool {
	for _, f := range r.Fields {
		if strings.TrimSpace(f) != "" {
			return false
		}
	}
	return true
}

// NormalizeHeader makes header matching forgiving: case, surrounding
// space, a stray BOM, and space/hyphen/underscore runs all fold away, so
// "Asset ID", "asset-id" and " ASSET_ID " all normalize to "asset_id".
func NormalizeHeader(h string) string {
	h = strings.TrimPrefix(strings.TrimSpace(h), byteOrderMark)
	parts := strings.FieldsFunc(strings.ToLower(h), func(r rune) bool {
		return r == ' ' || r == '-' || r == '_' || r == '\t'
	})
	return strings.Join(parts, "_")
}

// ParsedCSV is a structurally-valid CSV: a header and the rows beneath it.
type ParsedCSV struct {
	Header     []string
	HeaderLine int
	Rows       []Row
}

// Options configures the structural checks Parse performs. The zero
// value applies the loosest checks.
type Options struct {
	// MinColumns rejects a file whose header has fewer fields than
	// this. Zero (the default) disables the check. csvx has no idea how
	// many columns a valid row needs — the caller supplies this from
	// its own domain knowledge, purely as a cheap signal that the file
	// wasn't actually comma-delimited (a semicolon/tab file often
	// "succeeds" as one wide column per row otherwise).
	MinColumns int
}

// ErrorCode identifies the category of a structural CSV problem, so a
// caller can map it to a response without string-matching messages.
type ErrorCode string

const (
	ErrCodeMissingFile      ErrorCode = "missing_file"
	ErrCodeInvalidUpload    ErrorCode = "invalid_upload"
	ErrCodeInvalidExtension ErrorCode = "invalid_extension"
	ErrCodeEmptyFile        ErrorCode = "empty_file"
	ErrCodeMalformedCSV     ErrorCode = "malformed_csv"
)

// Error is a structural CSV problem — distinct from a per-row semantic
// rejection. It means the file should never even reach the semantic
// validator, not that one row inside an otherwise-valid file is bad.
// Callers errors.As for *Error and respond with a plain 4xx, never the
// per-row rejection shape.
type Error struct {
	Code ErrorCode
	Msg  string
	Err  error // wrapped cause (e.g. *csv.ParseError); nil where there is none
}

func (e *Error) Error() string { return e.Msg }
func (e *Error) Unwrap() error { return e.Err }

// Extract pulls the multipart file under fieldName out of r's form and
// returns it along with its header (which carries the original
// filename). The caller must Close() the returned file. All
// content-based validation (extension, emptiness, decodability) is
// Parse's job, not Extract's — Extract only does multipart mechanics.
//
// maxMemory is passed straight through to r.ParseMultipartForm (parts
// larger than this spill to a temp file); it is not a second size cap —
// router.MaxRequestBytes/middleware.RequestSizeLimit already bound the
// whole request body upstream, so nothing larger can ever arrive. Any
// error is wrapped with %w, not swallowed, so a caller can still detect
// a body-too-large failure via middleware.IsBodyTooLarge(err); a request that
// is not valid multipart is an *Error (ErrCodeInvalidUpload); csvx does
// not import middleware itself (base/ sits beneath it in the layering).
func Extract(r *http.Request, fieldName string, maxMemory int64) (multipart.File, *multipart.FileHeader, error) {
	if err := r.ParseMultipartForm(maxMemory); err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			return nil, nil, fmt.Errorf("csvx: parse multipart form: %w", err)
		}
		// Anything else is the client's request: not multipart at all, no
		// boundary, or a body cut short.
		return nil, nil, &Error{
			Code: ErrCodeInvalidUpload,
			Msg:  fmt.Sprintf("request must be multipart/form-data with a file under field %q", fieldName),
			Err:  err,
		}
	}
	file, header, err := r.FormFile(fieldName)
	if err != nil {
		if errors.Is(err, http.ErrMissingFile) {
			return nil, nil, &Error{
				Code: ErrCodeMissingFile,
				Msg:  fmt.Sprintf("no file provided under field %q", fieldName),
				Err:  err,
			}
		}
		return nil, nil, fmt.Errorf("csvx: read form file %q: %w", fieldName, err)
	}
	return file, header, nil
}

// Parse reads all of src, checks filename's extension and that the
// content is structurally valid CSV per opts, and returns the parsed
// rows with their true 1-indexed source line numbers, or an *Error
// describing the first structural problem found.
func Parse(src io.Reader, filename string, opts Options) (ParsedCSV, error) {
	if !strings.EqualFold(filepath.Ext(filename), ".csv") {
		return ParsedCSV{}, &Error{
			Code: ErrCodeInvalidExtension,
			Msg:  fmt.Sprintf("file %q must have a .csv extension", filename),
		}
	}

	data, err := io.ReadAll(src)
	if err != nil {
		return ParsedCSV{}, fmt.Errorf("csvx: read file: %w", err)
	}
	// Excel's "CSV UTF-8" export prefixes a byte-order mark; left in place
	// it would glue itself onto the first header name.
	data = bytes.TrimPrefix(data, utf8BOM)
	if len(data) == 0 {
		return ParsedCSV{}, &Error{Code: ErrCodeEmptyFile, Msg: "uploaded file is empty"}
	}
	if !utf8.Valid(data) {
		return ParsedCSV{}, &Error{Code: ErrCodeMalformedCSV, Msg: "file is not valid UTF-8 text"}
	}

	reader := csv.NewReader(bytes.NewReader(data))

	header, err := reader.Read()
	if err != nil {
		if errors.Is(err, io.EOF) {
			return ParsedCSV{}, &Error{Code: ErrCodeEmptyFile, Msg: "uploaded file has no header row"}
		}
		return ParsedCSV{}, malformedCSVError(err)
	}
	if opts.MinColumns > 0 && len(header) < opts.MinColumns {
		return ParsedCSV{}, &Error{
			Code: ErrCodeMalformedCSV,
			Msg: fmt.Sprintf(
				"header has only %d column(s), expected at least %d — check the file is comma-delimited",
				len(header), opts.MinColumns,
			),
		}
	}
	headerLine, _ := reader.FieldPos(0)

	var rows []Row
	for {
		record, err := reader.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return ParsedCSV{}, malformedCSVError(err)
		}
		line, _ := reader.FieldPos(0)
		rows = append(rows, Row{Line: line, Fields: record})
	}

	return ParsedCSV{Header: header, HeaderLine: headerLine, Rows: rows}, nil
}

func malformedCSVError(err error) *Error {
	return &Error{Code: ErrCodeMalformedCSV, Msg: fmt.Sprintf("file is not valid CSV: %s", err), Err: err}
}

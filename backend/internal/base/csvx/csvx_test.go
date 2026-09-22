package csvx

import (
	"bytes"
	"encoding/csv"
	"errors"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestParse_StripsUTF8BOM(t *testing.T) {
	got, err := Parse(strings.NewReader("\xef\xbb\xbfid,name\r\n1,alpha\r\n"), "assets.csv", Options{})
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if got.Header[0] != "id" {
		t.Errorf("Header[0] = %q, want %q (BOM must not leak into the header)", got.Header[0], "id")
	}
	if len(got.Rows) != 1 || got.Rows[0].Fields[1] != "alpha" {
		t.Errorf("Rows = %+v, want one row with CRLF endings handled", got.Rows)
	}
}

func TestParse_BOMOnlyFileIsEmpty(t *testing.T) {
	_, err := Parse(strings.NewReader("\xef\xbb\xbf"), "assets.csv", Options{})
	var ce *Error
	if !errors.As(err, &ce) || ce.Code != ErrCodeEmptyFile {
		t.Fatalf("Parse() error = %v, want ErrCodeEmptyFile", err)
	}
}

func TestParse_ValidCSV(t *testing.T) {
	tests := []struct {
		name           string
		content        string
		opts           Options
		wantHeader     []string
		wantHeaderLine int
		wantRows       []Row
	}{
		{
			name:           "multi-row CSV has sequential line numbers",
			content:        "id,name\n1,alpha\n2,beta\n3,gamma\n",
			wantHeader:     []string{"id", "name"},
			wantHeaderLine: 1,
			wantRows: []Row{
				{Line: 2, Fields: []string{"1", "alpha"}},
				{Line: 3, Fields: []string{"2", "beta"}},
				{Line: 4, Fields: []string{"3", "gamma"}},
			},
		},
		{
			name:           "quoted multi-line field advances line numbers correctly",
			content:        "id,note\nval1,\"line1\nline2\"\nval3,val4\n",
			wantHeader:     []string{"id", "note"},
			wantHeaderLine: 1,
			wantRows: []Row{
				{Line: 2, Fields: []string{"val1", "line1\nline2"}},
				// The quoted field above spans physical lines 2-3, so the
				// next record starts at line 4, not line 3 — this is
				// exactly what a naive per-record counter would get wrong.
				{Line: 4, Fields: []string{"val3", "val4"}},
			},
		},
		{
			name:           "blank line mid-file does not corrupt later line numbers",
			content:        "id,name\nval1,val2\n\nval3,val4\n",
			wantHeader:     []string{"id", "name"},
			wantHeaderLine: 1,
			wantRows: []Row{
				{Line: 2, Fields: []string{"val1", "val2"}},
				// encoding/csv silently skips the blank line 3, so the
				// next record is on line 4 — again wrong under a naive
				// counter, which would land on line 3.
				{Line: 4, Fields: []string{"val3", "val4"}},
			},
		},
		{
			name:           "header-only file succeeds with no rows",
			content:        "id,name\n",
			wantHeader:     []string{"id", "name"},
			wantHeaderLine: 1,
			wantRows:       nil,
		},
		{
			name:           "MinColumns unset accepts a narrow header",
			content:        "id;name;type\n",
			opts:           Options{},
			wantHeader:     []string{"id;name;type"},
			wantHeaderLine: 1,
			wantRows:       nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Parse(strings.NewReader(tt.content), "upload.csv", tt.opts)
			if err != nil {
				t.Fatalf("Parse() error = %v, want nil", err)
			}
			if !slicesEqual(got.Header, tt.wantHeader) {
				t.Errorf("Header = %v, want %v", got.Header, tt.wantHeader)
			}
			if got.HeaderLine != tt.wantHeaderLine {
				t.Errorf("HeaderLine = %d, want %d", got.HeaderLine, tt.wantHeaderLine)
			}
			if !rowsEqual(got.Rows, tt.wantRows) {
				t.Errorf("Rows = %+v, want %+v", got.Rows, tt.wantRows)
			}
		})
	}
}

func TestParse_StructuralErrors(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		content  []byte
		opts     Options
		wantCode ErrorCode
	}{
		{
			name:     "non-.csv extension rejected",
			filename: "upload.txt",
			content:  []byte("id,name\n1,alpha\n"),
			wantCode: ErrCodeInvalidExtension,
		},
		{
			name:     "extension check is case-insensitive (accepted)",
			filename: "UPLOAD.CSV",
			content:  []byte("id,name\n1,alpha\n"),
			wantCode: "", // no error expected; see assertion below
		},
		{
			name:     "zero-byte file rejected",
			filename: "upload.csv",
			content:  []byte(""),
			wantCode: ErrCodeEmptyFile,
		},
		{
			name:     "file of only blank lines rejected",
			filename: "upload.csv",
			content:  []byte("\n\n\n"),
			wantCode: ErrCodeEmptyFile,
		},
		{
			name:     "ragged row rejected",
			filename: "upload.csv",
			content:  []byte("id,name\n1,alpha,extra\n"),
			wantCode: ErrCodeMalformedCSV,
		},
		{
			name:     "bad quoting rejected",
			filename: "upload.csv",
			content:  []byte("id,name\n1,\"alpha\"bogus\n"),
			wantCode: ErrCodeMalformedCSV,
		},
		{
			name:     "invalid UTF-8 rejected",
			filename: "upload.csv",
			content:  []byte("id,name\n1,\xff\xfe\n"),
			wantCode: ErrCodeMalformedCSV,
		},
		{
			name:     "MinColumns rejects a too-narrow header",
			filename: "upload.csv",
			content:  []byte("id;name;type\n"),
			opts:     Options{MinColumns: 2},
			wantCode: ErrCodeMalformedCSV,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Parse(bytes.NewReader(tt.content), tt.filename, tt.opts)
			if tt.wantCode == "" {
				if err != nil {
					t.Fatalf("Parse() error = %v, want nil", err)
				}
				return
			}
			var csvxErr *Error
			if !errors.As(err, &csvxErr) {
				t.Fatalf("Parse() error = %v, want *csvx.Error", err)
			}
			if csvxErr.Code != tt.wantCode {
				t.Errorf("Code = %q, want %q", csvxErr.Code, tt.wantCode)
			}
		})
	}
}

func TestParse_RaggedRowPreservesUnderlyingCSVError(t *testing.T) {
	_, err := Parse(strings.NewReader("id,name\n1,alpha,extra\n"), "upload.csv", Options{})
	var csvxErr *Error
	if !errors.As(err, &csvxErr) {
		t.Fatalf("error = %v, want *csvx.Error", err)
	}
	var parseErr *csv.ParseError
	if !errors.As(csvxErr, &parseErr) {
		t.Fatalf("expected wrapped *csv.ParseError to be recoverable via errors.As, got: %v", csvxErr.Unwrap())
	}
}

func TestExtract_ValidUpload(t *testing.T) {
	req := newMultipartRequest(t, "file", "upload.csv", "id,name\n1,alpha\n")

	file, header, err := Extract(req, "file", 1<<20)
	if err != nil {
		t.Fatalf("Extract() error = %v, want nil", err)
	}
	defer file.Close()

	if header.Filename != "upload.csv" {
		t.Errorf("Filename = %q, want %q", header.Filename, "upload.csv")
	}
	got, err := readAllString(file)
	if err != nil {
		t.Fatalf("read extracted file: %v", err)
	}
	want := "id,name\n1,alpha\n"
	if got != want {
		t.Errorf("file contents = %q, want %q", got, want)
	}
}

func TestExtract_MissingField(t *testing.T) {
	req := newMultipartRequest(t, "wrong_field", "upload.csv", "id,name\n1,alpha\n")

	_, _, err := Extract(req, "file", 1<<20)
	var csvxErr *Error
	if !errors.As(err, &csvxErr) {
		t.Fatalf("Extract() error = %v, want *csvx.Error", err)
	}
	if csvxErr.Code != ErrCodeMissingFile {
		t.Errorf("Code = %q, want %q", csvxErr.Code, ErrCodeMissingFile)
	}
}

func TestExtract_NotMultipart(t *testing.T) {
	tests := []struct {
		name        string
		contentType string
		body        string
	}{
		{"no content type", "", ""},
		{"json body", "application/json", `{"a":1}`},
		{"multipart without boundary", "multipart/form-data", "garbage"},
		{"garbage under a boundary", "multipart/form-data; boundary=xx", "garbage"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(tc.body))
			if tc.contentType != "" {
				req.Header.Set("Content-Type", tc.contentType)
			}
			_, _, err := Extract(req, "file", 1<<20)
			var csvxErr *Error
			if !errors.As(err, &csvxErr) {
				t.Fatalf("Extract() error = %v, want *csvx.Error", err)
			}
			if csvxErr.Code != ErrCodeInvalidUpload {
				t.Errorf("Code = %q, want %q", csvxErr.Code, ErrCodeInvalidUpload)
			}
		})
	}
}

func TestExtract_DoesNotSwallowBodyTooLarge(t *testing.T) {
	req := newMultipartRequest(t, "file", "upload.csv", strings.Repeat("id,name\n1,alpha\n", 100))
	rec := httptest.NewRecorder()
	req.Body = http.MaxBytesReader(rec, req.Body, 10)

	_, _, err := Extract(req, "file", 1<<20)
	if err == nil {
		t.Fatal("Extract() error = nil, want a body-too-large error")
	}
	var mbe *http.MaxBytesError
	if !errors.As(err, &mbe) {
		t.Fatalf("Extract() error = %v, want it to wrap *http.MaxBytesError", err)
	}
}

func newMultipartRequest(t *testing.T, fieldName, filename, content string) *http.Request {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	part, err := w.CreateFormFile(fieldName, filename)
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	if _, err := part.Write([]byte(content)); err != nil {
		t.Fatalf("write form file: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/", &buf)
	req.Header.Set("Content-Type", w.FormDataContentType())
	return req
}

func readAllString(r multipart.File) (string, error) {
	var buf bytes.Buffer
	_, err := buf.ReadFrom(r)
	return buf.String(), err
}

func slicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func rowsEqual(a, b []Row) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].Line != b[i].Line || !slicesEqual(a[i].Fields, b[i].Fields) {
			return false
		}
	}
	return true
}

func TestRow_IsBlank(t *testing.T) {
	tests := []struct {
		name   string
		fields []string
		want   bool
	}{
		{"all empty", []string{"", "", ""}, true},
		{"whitespace only", []string{" ", "\t", "  "}, true},
		{"no fields", nil, true},
		{"one value", []string{"", "x", ""}, false},
		{"value with padding", []string{"  a  "}, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := (Row{Fields: tc.fields}).IsBlank(); got != tc.want {
				t.Errorf("IsBlank(%q) = %v, want %v", tc.fields, got, tc.want)
			}
		})
	}
}

func TestNormalizeHeader(t *testing.T) {
	tests := []struct{ in, want string }{
		{"asset_id", "asset_id"},
		{"Asset ID", "asset_id"},
		{"ASSET-ID", "asset_id"},
		{" Asset_Type ", "asset_type"},
		{"asset  __ name", "asset_name"},
		{"\uFEFFasset_id", "asset_id"},
		{"\uFEFF Asset ID", "asset_id"},
		{"", ""},
		{"   ", ""},
	}
	for _, tc := range tests {
		if got := NormalizeHeader(tc.in); got != tc.want {
			t.Errorf("NormalizeHeader(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"smart-grid-asset-management/backend/internal/base/errx"
	"smart-grid-asset-management/backend/internal/base/logs"
	"smart-grid-asset-management/backend/internal/defined"
	"smart-grid-asset-management/backend/internal/entity/dto"
	"smart-grid-asset-management/backend/internal/middleware"
	"smart-grid-asset-management/backend/internal/service"
)

const goodCSV = "asset_id,asset_type,asset_name,operational_status\nSUB-1,SUBSTATION,Sub,IN_SERVICE\n"

type fakeImportService struct {
	importFn  func(ctx context.Context, in service.ImportInput) (dto.ImportResponse, error)
	previewFn func(ctx context.Context, in service.ImportInput) (dto.ImportPreviewResponse, error)
	got       service.ImportInput
	getFn     func(ctx context.Context, id string) (dto.ImportResponse, error)
	listFn    func(req dto.ImportListRequest) (dto.ImportListResponse, error)
}

func (f *fakeImportService) Preview(ctx context.Context, in service.ImportInput) (dto.ImportPreviewResponse, error) {
	f.got = in
	return f.previewFn(ctx, in)
}
func (f *fakeImportService) ListImports(_ context.Context, req dto.ImportListRequest) (dto.ImportListResponse, error) {
	return f.listFn(req)
}
func (f *fakeImportService) Template() []byte { return []byte("asset_id,asset_type\n") }

func (f *fakeImportService) Import(ctx context.Context, in service.ImportInput) (dto.ImportResponse, error) {
	f.got = in
	return f.importFn(ctx, in)
}
func (f *fakeImportService) GetImport(ctx context.Context, req dto.ImportIDRequest) (dto.ImportResponse, error) {
	return f.getFn(ctx, req.ImportID)
}

func multipartRequest(t *testing.T, field, filename, content string) *http.Request {
	t.Helper()
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	if field != "" {
		fw, err := mw.CreateFormFile(field, filename)
		if err != nil {
			t.Fatal(err)
		}
		_, _ = io.WriteString(fw, content)
	}
	_ = mw.Close()
	req := httptest.NewRequest(http.MethodPost, "/api/imports", &body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.RemoteAddr = "192.0.2.10:5555"
	return req
}

func init() { gin.SetMode(gin.TestMode) }

// serve runs h on a gin context (with the given path params and a trace ID
// in the request context) and returns the response plus captured logs.
func serve(t *testing.T, h gin.HandlerFunc, req *http.Request, params ...gin.Param) (*httptest.ResponseRecorder, string) {
	t.Helper()
	var logBuf bytes.Buffer
	logs.SetOutput(&logBuf)
	t.Cleanup(func() { logs.SetOutput(os.Stdout) })
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = req
	c.Params = params
	middleware.TraceID(c)
	h(c)
	return rec, logBuf.String()
}

func decodeError(t *testing.T, rec *httptest.ResponseRecorder) dto.ErrorResponse {
	t.Helper()
	var e dto.ErrorResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &e); err != nil {
		t.Fatalf("body is not an ErrorResponse: %v (%q)", err, rec.Body.String())
	}
	return e
}

func TestImportHandler_Create_Success(t *testing.T) {
	svc := &fakeImportService{importFn: func(context.Context, service.ImportInput) (dto.ImportResponse, error) {
		return dto.ImportResponse{ImportID: "abc-123", TotalRows: 1, ImportedRows: 1, Committed: true, Rejections: []dto.Rejection{}, IgnoredColumns: []string{}}, nil
	}}
	h := NewImportHandler(svc, 1<<20)
	req := multipartRequest(t, "file", "Assets.CSV", goodCSV)
	req.Header.Set("X-Forwarded-For", "203.0.113.9, 198.51.100.4")

	rec, _ := serve(t, h.Create, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body %s", rec.Code, rec.Body.String())
	}
	if loc := rec.Header().Get("Location"); loc != "/api/imports/abc-123" {
		t.Errorf("Location = %q", loc)
	}
	var resp dto.ImportResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil || !resp.Committed || resp.TotalRows != 1 {
		t.Errorf("body = %s (%v)", rec.Body.String(), err)
	}
	if svc.got.Filename != "Assets.CSV" || svc.got.ClientIP != "198.51.100.4" {
		t.Errorf("service input = filename %q ip %q; want the uploaded name and the last forwarded hop", svc.got.Filename, svc.got.ClientIP)
	}
	if len(svc.got.CSV.Rows) != 1 || svc.got.CSV.Rows[0].Line != 2 {
		t.Errorf("parsed CSV handed to service = %+v", svc.got.CSV)
	}
}

func TestImportHandler_Create_ErrorMapping(t *testing.T) {
	const secret = "pq: password authentication failed for user grid"
	tests := []struct {
		name       string
		req        func(t *testing.T) *http.Request
		serviceErr error
		wantStatus int
		wantMsg    string // substring of the client-visible message
		wantLine   int    // expected `line` (0 = none)
	}{
		{"no file field", func(t *testing.T) *http.Request { return multipartRequest(t, "", "", "") }, nil, 400, "no file provided", 0},
		{"wrong extension", func(t *testing.T) *http.Request { return multipartRequest(t, "file", "assets.txt", goodCSV) }, nil, 415, "must have a .csv extension", 0},
		{"empty file", func(t *testing.T) *http.Request { return multipartRequest(t, "file", "a.csv", "") }, nil, 400, "empty", 0},
		{"ragged row reports its line", func(t *testing.T) *http.Request {
			return multipartRequest(t, "file", "a.csv", "asset_id,asset_type,asset_name,operational_status\nA,B,C,D\nA,B\n")
		}, nil, 422, "not valid CSV", 3},
		{"too few columns (wrong delimiter)", func(t *testing.T) *http.Request {
			return multipartRequest(t, "file", "a.csv", "asset_id;asset_type;asset_name;operational_status\nA;B;C;D\n")
		}, nil, 422, "comma-delimited", 0},
		{"schema error from service", func(t *testing.T) *http.Request { return multipartRequest(t, "file", "a.csv", goodCSV) },
			&service.SchemaError{Missing: []string{"asset_type"}, Found: []string{"asset_id"}}, 422, "missing required column(s): asset_type", 0},
		{"no data rows", func(t *testing.T) *http.Request { return multipartRequest(t, "file", "a.csv", goodCSV) }, service.ErrNoDataRows, 422, "no data rows", 0},
		{"conflict", func(t *testing.T) *http.Request { return multipartRequest(t, "file", "a.csv", goodCSV) }, fmt.Errorf("commit: %w", errx.ErrConflict), 409, "please retry", 0},
		{"unexpected error is generic", func(t *testing.T) *http.Request { return multipartRequest(t, "file", "a.csv", goodCSV) }, errors.New(secret), 500, "internal server error", 0},
		{"body exceeds the size limit", func(t *testing.T) *http.Request {
			req := multipartRequest(t, "file", "a.csv", goodCSV+strings.Repeat("x", 4096))
			req.Body = http.MaxBytesReader(httptest.NewRecorder(), req.Body, 512)
			return req
		}, nil, 413, "too large", 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			svc := &fakeImportService{importFn: func(context.Context, service.ImportInput) (dto.ImportResponse, error) {
				return dto.ImportResponse{}, tc.serviceErr
			}}
			req := tc.req(t)
			req.Header.Set(defined.TraceIDHeader, "trace-77")
			rec, logged := serve(t, NewImportHandler(svc, 1<<20).Create, req)

			if rec.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d; body %s", rec.Code, tc.wantStatus, rec.Body.String())
			}
			e := decodeError(t, rec)
			if !strings.Contains(e.Error, tc.wantMsg) {
				t.Errorf("error = %q, want it to contain %q", e.Error, tc.wantMsg)
			}
			if e.TraceID != "trace-77" {
				t.Errorf("trace_id = %q, want trace-77", e.TraceID)
			}
			if tc.wantLine == 0 && e.Line != nil {
				t.Errorf("line = %d, want none", *e.Line)
			}
			if tc.wantLine != 0 && (e.Line == nil || *e.Line != tc.wantLine) {
				t.Errorf("line = %v, want %d", e.Line, tc.wantLine)
			}
			if !strings.Contains(logged, "tid=trace-77") {
				t.Errorf("log line missing the trace id:\n%s", logged)
			}
			if tc.wantStatus == 500 {
				if strings.Contains(rec.Body.String(), "password") {
					t.Errorf("500 leaks internals to the client: %s", rec.Body.String())
				}
				if !strings.Contains(logged, "lvl=ERROR") || !strings.Contains(logged, "password authentication failed") {
					t.Errorf("500 must log the real error at Error level:\n%s", logged)
				}
			} else if !strings.Contains(logged, "lvl=WARN") {
				t.Errorf("client errors are logged at Warn:\n%s", logged)
			}
			if tc.wantLine != 0 && !strings.Contains(logged, fmt.Sprintf("line=%d", tc.wantLine)) {
				t.Errorf("malformed-CSV log line must name the CSV line:\n%s", logged)
			}
		})
	}
}

func TestImportHandler_Create_SanitizesTheFilename(t *testing.T) {
	svc := &fakeImportService{importFn: func(context.Context, service.ImportInput) (dto.ImportResponse, error) {
		return dto.ImportResponse{ImportID: "x"}, nil
	}}
	req := multipartRequest(t, "file", "..\\..\\evil\nname.csv", goodCSV)
	serve(t, NewImportHandler(svc, 1<<20).Create, req)
	if strings.ContainsAny(svc.got.Filename, "\n\\/") || !strings.HasSuffix(svc.got.Filename, "name.csv") {
		t.Errorf("Filename = %q, want a plain base name without separators or newlines", svc.got.Filename)
	}
	if got := sanitizeFilename(strings.Repeat("é", 400) + ".csv"); len([]rune(got)) != defined.MaxFilenameRunes {
		t.Errorf("long filename kept %d runes, want %d", len([]rune(got)), defined.MaxFilenameRunes)
	}
}

func TestImportHandler_Get(t *testing.T) {
	svc := &fakeImportService{getFn: func(_ context.Context, id string) (dto.ImportResponse, error) {
		switch id {
		case "found":
			return dto.ImportResponse{ImportID: "found", Rejections: []dto.Rejection{}}, nil
		case "bad":
			return dto.ImportResponse{}, &errx.InputError{Msg: `import id "bad" is not a valid UUID`}
		}
		return dto.ImportResponse{}, errx.ErrNotFound
	}}
	h := NewImportHandler(svc, 1<<20)
	for id, want := range map[string]int{"found": 200, "bad": 400, "gone": 404} {
		req := httptest.NewRequest(http.MethodGet, "/api/imports/"+id, nil)
		if rec, _ := serve(t, h.Get, req, gin.Param{Key: "importId", Value: id}); rec.Code != want {
			t.Errorf("GET %s = %d, want %d", id, rec.Code, want)
		}
	}
}

type fakeAssetService struct {
	search    func(q, typ string, limit int) (dto.SearchResponse, error)
	getErr    error
	list      func(req dto.ListRequest) (dto.AssetListResponse, error)
	deleteErr error
	deleted   string
}

func (f *fakeAssetService) List(_ context.Context, req dto.ListRequest) (dto.AssetListResponse, error) {
	return f.list(req)
}
func (f *fakeAssetService) Stats(context.Context) (dto.StatsResponse, error) {
	return dto.StatsResponse{ByType: []dto.TypeCount{}, ByStatus: []dto.StatusCount{}}, f.getErr
}
func (f *fakeAssetService) Delete(_ context.Context, id string) error {
	f.deleted = id
	return f.deleteErr
}
func (f *fakeAssetService) Roots(context.Context) (dto.RootsResponse, error) {
	return dto.RootsResponse{Roots: []dto.AssetNode{}}, nil
}
func (f *fakeAssetService) Get(_ context.Context, id string) (dto.AssetDetail, error) {
	return dto.AssetDetail{AssetID: id}, f.getErr
}
func (f *fakeAssetService) Children(_ context.Context, id string) (dto.ChildrenResponse, error) {
	return dto.ChildrenResponse{AssetID: id}, f.getErr
}
func (f *fakeAssetService) Ancestors(_ context.Context, id string) (dto.AncestorsResponse, error) {
	return dto.AncestorsResponse{AssetID: id}, f.getErr
}
func (f *fakeAssetService) Search(_ context.Context, req dto.SearchRequest) (dto.SearchResponse, error) {
	return f.search(req.Q, req.Type, req.Limit)
}

func TestAssetHandler_NotFoundBecomes404JSON(t *testing.T) {
	h := NewAssetHandler(&fakeAssetService{getErr: fmt.Errorf("get asset: %w", errx.ErrNotFound)})
	for name, fn := range map[string]gin.HandlerFunc{"get": h.Get, "children": h.Children, "ancestors": h.Ancestors} {
		req := httptest.NewRequest(http.MethodGet, "/x", nil)
		rec, _ := serve(t, fn, req, gin.Param{Key: "assetId", Value: "NOPE"})
		if rec.Code != http.StatusNotFound || decodeError(t, rec).Error != "not found" {
			t.Errorf("%s: %d %s, want 404 {error: not found}", name, rec.Code, rec.Body.String())
		}
		if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
			t.Errorf("%s: Content-Type = %q", name, ct)
		}
	}
}

func TestAssetHandler_SearchParameters(t *testing.T) {
	var gotQ, gotType string
	var gotLimit int
	h := NewAssetHandler(&fakeAssetService{search: func(q, typ string, limit int) (dto.SearchResponse, error) {
		gotQ, gotType, gotLimit = q, typ, limit
		if q == "" {
			return dto.SearchResponse{}, &errx.InputError{Msg: "query parameter q is required"}
		}
		return dto.SearchResponse{Query: q, Results: []dto.AssetSummary{}}, nil
	}})

	rec, _ := serve(t, h.Search, httptest.NewRequest(http.MethodGet, "/api/assets/search?q=tx%20%25&type=TRANSFORMER&limit=25", nil))
	if rec.Code != 200 || gotQ != "tx %" || gotType != "TRANSFORMER" || gotLimit != 25 {
		t.Errorf("status %d, passed q=%q type=%q limit=%d", rec.Code, gotQ, gotType, gotLimit)
	}
	if rec, _ := serve(t, h.Search, httptest.NewRequest(http.MethodGet, "/api/assets/search?q=a&limit=many", nil)); rec.Code != 400 {
		t.Errorf("non-numeric limit: status %d, want 400", rec.Code)
	}
	rec, _ = serve(t, h.Search, httptest.NewRequest(http.MethodGet, "/api/assets/search", nil))
	if rec.Code != 400 || !strings.Contains(decodeError(t, rec).Error, "q is required") {
		t.Errorf("missing q: %d %s", rec.Code, rec.Body.String())
	}
}

type fakeSnapshots struct{ snap *service.LookupSnapshot }

func (f fakeSnapshots) Snapshot() *service.LookupSnapshot { return f.snap }

func TestLookupHandler_ReturnsNonNullLists(t *testing.T) {
	cache := service.NewLookupCache(nil) // never refreshed: an empty snapshot
	rec, _ := serve(t, NewLookupHandler(fakeSnapshots{cache.Snapshot()}).List, httptest.NewRequest(http.MethodGet, "/api/lookups", nil))
	if rec.Code != 200 {
		t.Fatalf("status = %d", rec.Code)
	}
	for _, field := range []string{`"asset_types":[]`, `"operational_statuses":[]`, `"parent_rules":[]`, `"root_types":[]`} {
		if !strings.Contains(rec.Body.String(), field) {
			t.Errorf("body %s should contain %s (empty lists serialise as [], never null)", rec.Body.String(), field)
		}
	}
}

func TestImportHandler_PreviewAndConfirmForwardTheFingerprint(t *testing.T) {
	svc := &fakeImportService{
		previewFn: func(context.Context, service.ImportInput) (dto.ImportPreviewResponse, error) {
			return dto.ImportPreviewResponse{TotalRows: 1, ImportableRows: 1, Rejections: []dto.Rejection{}, IgnoredColumns: []string{}, Fingerprint: "fp1"}, nil
		},
		importFn: func(_ context.Context, in service.ImportInput) (dto.ImportResponse, error) {
			if in.ExpectedFingerprint != "fp1" {
				return dto.ImportResponse{}, service.ErrPreviewStale
			}
			return dto.ImportResponse{ImportID: "id", Rejections: []dto.Rejection{}, IgnoredColumns: []string{}}, nil
		},
	}
	h := NewImportHandler(svc, 1<<20)

	rec, _ := serve(t, h.Preview, multipartRequest(t, "file", "a.csv", goodCSV))
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"fingerprint":"fp1"`) {
		t.Fatalf("preview: %d %s", rec.Code, rec.Body.String())
	}

	// The optional form field reaches the service; a mismatch is a 412 with the service's message.
	withFP := multipartRequestWithField(t, "expected_fingerprint", " fp1 ")
	if rec, _ := serve(t, h.Create, withFP); rec.Code != 200 || svc.got.ExpectedFingerprint != "fp1" {
		t.Errorf("matching fingerprint: %d, service saw %q", rec.Code, svc.got.ExpectedFingerprint)
	}
	rec, _ = serve(t, h.Create, multipartRequestWithField(t, "expected_fingerprint", "old"))
	if rec.Code != http.StatusPreconditionFailed || !strings.Contains(decodeError(t, rec).Error, "review it again") {
		t.Errorf("stale fingerprint: %d %s, want 412", rec.Code, rec.Body.String())
	}
}

func multipartRequestWithField(t *testing.T, name, value string) *http.Request {
	t.Helper()
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	_ = mw.WriteField(name, value)
	fw, _ := mw.CreateFormFile("file", "a.csv")
	_, _ = io.WriteString(fw, goodCSV)
	_ = mw.Close()
	req := httptest.NewRequest(http.MethodPost, "/api/imports", &body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	return req
}

func TestImportHandler_TemplateAndList(t *testing.T) {
	svc := &fakeImportService{listFn: func(req dto.ImportListRequest) (dto.ImportListResponse, error) {
		if req.Limit == 999 {
			return dto.ImportListResponse{}, &errx.InputError{Msg: "limit must be between 1 and 100"}
		}
		return dto.ImportListResponse{Limit: req.Limit, Offset: req.Offset, Items: []dto.ImportSummary{}}, nil
	}}
	h := NewImportHandler(svc, 1<<20)

	rec, _ := serve(t, h.Template, httptest.NewRequest(http.MethodGet, "/api/imports/template", nil))
	if rec.Code != 200 || !strings.HasPrefix(rec.Header().Get("Content-Type"), "text/csv") ||
		rec.Header().Get("Content-Disposition") != `attachment; filename="asset_import_template.csv"` {
		t.Errorf("template: %d %v", rec.Code, rec.Header())
	}
	rec, _ = serve(t, h.List, httptest.NewRequest(http.MethodGet, "/api/imports?limit=5&offset=10", nil))
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"limit":5`) || !strings.Contains(rec.Body.String(), `"offset":10`) {
		t.Errorf("list: %d %s", rec.Code, rec.Body.String())
	}
	for path, want := range map[string]int{"/api/imports?limit=999": 400, "/api/imports?limit=x": 400} {
		if rec, _ := serve(t, h.List, httptest.NewRequest(http.MethodGet, path, nil)); rec.Code != want {
			t.Errorf("GET %s = %d, want %d", path, rec.Code, want)
		}
	}
}

func TestAssetHandler_DeleteStatusMapping(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want int
		msg  string
	}{
		{"deleted", nil, http.StatusNoContent, ""},
		{"unknown asset", fmt.Errorf("delete: %w", errx.ErrNotFound), http.StatusNotFound, "not found"},
		{"has children", errx.Conflict("asset %q has %d child asset(s); delete them first", "SUB-1", 3), http.StatusConflict, "3 child asset(s)"},
		{"lost a race", fmt.Errorf("%w: fk", errx.ErrConflict), http.StatusConflict, "changed while"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			svc := &fakeAssetService{deleteErr: tc.err}
			rec, logged := serve(t, NewAssetHandler(svc).Delete, httptest.NewRequest(http.MethodDelete, "/api/assets/SUB-1", nil), gin.Param{Key: "assetId", Value: "SUB-1"})
			if rec.Code != tc.want || svc.deleted != "SUB-1" {
				t.Fatalf("status %d (service saw %q), want %d", rec.Code, svc.deleted, tc.want)
			}
			if tc.msg != "" && !strings.Contains(decodeError(t, rec).Error, tc.msg) {
				t.Errorf("body %s should mention %q", rec.Body.String(), tc.msg)
			}
			if tc.err == nil && (!strings.Contains(logged, "asset deleted") || !strings.Contains(logged, "asset_id=SUB-1")) {
				t.Errorf("a delete must be logged, got %q", logged)
			}
		})
	}
}

func TestAssetHandler_ListParameters(t *testing.T) {
	var got dto.ListRequest
	h := NewAssetHandler(&fakeAssetService{list: func(req dto.ListRequest) (dto.AssetListResponse, error) {
		got = req
		return dto.AssetListResponse{Items: []dto.AssetSummary{}}, nil
	}})
	rec, _ := serve(t, h.List, httptest.NewRequest(http.MethodGet, "/api/assets?q=tx&type=TRANSFORMER&status=IN_SERVICE&limit=10&offset=20", nil))
	if rec.Code != 200 || got != (dto.ListRequest{Q: "tx", Type: "TRANSFORMER", Status: "IN_SERVICE", Limit: 10, Offset: 20}) {
		t.Errorf("status %d, service got %+v", rec.Code, got)
	}
	if rec, _ := serve(t, h.List, httptest.NewRequest(http.MethodGet, "/api/assets?offset=abc", nil)); rec.Code != 400 {
		t.Errorf("non-numeric offset: %d, want 400", rec.Code)
	}
}

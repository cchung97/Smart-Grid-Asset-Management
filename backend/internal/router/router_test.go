package router_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"smart-grid-asset-management/backend/internal/base/logs"
	"smart-grid-asset-management/backend/internal/entity/dto"
	"smart-grid-asset-management/backend/internal/handler"
	"smart-grid-asset-management/backend/internal/middleware"
	"smart-grid-asset-management/backend/internal/repository"
	"smart-grid-asset-management/backend/internal/router"
	"smart-grid-asset-management/backend/internal/service"
	"smart-grid-asset-management/backend/internal/testutil"
)

const header = "asset_id,asset_type,asset_name,operational_status,parent_asset_id,voltage_kv,rating_kva,manufacturer,model,serial_number,commissioned_date\n"

const goodGrid = header + `
SWP-1,SWITCHBOARD_PANEL,Panel 1,IN_SERVICE,SWB-1,,,,,,
SWB-1,SWITCHBOARD,Switchboard 1,IN_SERVICE,SUB-1,,,,,,
TX-1,TRANSFORMER,Transformer 1,MAINTENANCE,SUB-1,22,1000,ABB,R1,SN1,2019-04-30
SUB-1,SUBSTATION,Substation 1,IN_SERVICE,,66,,,,,
`

const testKey = "test-key"

// newServer builds the real router over a real (isolated-schema) database,
// configured with testKey as the API key that guards every /api endpoint.
func newServer(t *testing.T) *httptest.Server { return newServerWithKey(t, testKey) }

func newServerWithKey(t *testing.T, apiKey string) *httptest.Server {
	t.Helper()
	logs.SetOutput(io.Discard)
	t.Cleanup(func() { logs.SetOutput(os.Stdout) })

	db := testutil.NewTestDB(t)
	lookups := service.NewLookupCache(repository.NewLookupRepository(db))
	if err := lookups.Refresh(context.Background()); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	const maxBytes = 1 << 20
	srv := httptest.NewServer(router.New(router.Deps{
		Health:          handler.NewHealthHandler(service.NewHealthService(db)),
		Imports:         handler.NewImportHandler(service.NewImportService(db, lookups), maxBytes),
		Assets:          handler.NewAssetHandler(service.NewAssetService(repository.NewAssetRepository(db), lookups)),
		Lookups:         handler.NewLookupHandler(lookups),
		APIKey:          apiKey,
		RateLimiter:     middleware.NewRateLimiter(ctx, 1000, 0),
		MaxRequestBytes: maxBytes,
	}))
	t.Cleanup(srv.Close)
	return srv
}

func upload(t *testing.T, srv *httptest.Server, filename, content string) *http.Response {
	t.Helper()
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	fw, _ := mw.CreateFormFile("file", filename)
	_, _ = io.WriteString(fw, content)
	_ = mw.Close()
	req, _ := http.NewRequest(http.MethodPost, srv.URL+"/api/imports", &body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set("x-api-key", testKey)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	return resp
}

func decode[T any](t *testing.T, resp *http.Response) T {
	t.Helper()
	defer resp.Body.Close()
	var v T
	if err := json.NewDecoder(resp.Body).Decode(&v); err != nil {
		t.Fatalf("decode %s: %v", resp.Request.URL.Path, err)
	}
	return v
}

func get(t *testing.T, srv *httptest.Server, path string) *http.Response {
	t.Helper()
	return do(t, srv, http.MethodGet, path)
}

func TestAPI_ImportThenExplore(t *testing.T) {
	srv := newServer(t)

	resp := upload(t, srv, "grid.csv", goodGrid)
	if resp.StatusCode != 200 || resp.Header.Get("X-Trace-Id") == "" {
		t.Fatalf("upload status %d, trace header %q", resp.StatusCode, resp.Header.Get("X-Trace-Id"))
	}
	loc := resp.Header.Get("Location")
	imp := decode[dto.ImportResponse](t, resp)
	if !imp.Committed || imp.TotalRows != 4 || imp.ImportedRows != 4 || loc != "/api/imports/"+imp.ImportID {
		t.Fatalf("import = %+v, Location %q", imp, loc)
	}

	again := decode[dto.ImportResponse](t, get(t, srv, loc))
	if again.ImportID != imp.ImportID || !again.Committed {
		t.Errorf("GET %s = %+v", loc, again)
	}

	roots := decode[dto.RootsResponse](t, get(t, srv, "/api/assets/roots"))
	if roots.Total != 1 || roots.Roots[0].AssetID != "SUB-1" || roots.Roots[0].SubtreeCount != 4 {
		t.Errorf("roots = %+v", roots)
	}
	kids := decode[dto.ChildrenResponse](t, get(t, srv, "/api/assets/SUB-1/children"))
	if kids.TotalChildren != 2 || len(kids.Groups) != 2 || kids.Groups[0].AssetType != "SWITCHBOARD" {
		t.Errorf("children = %+v", kids)
	}
	// the sortable columns come with each child, so the UI needs no request per row
	for _, g := range kids.Groups {
		for _, n := range g.Assets {
			switch n.AssetID {
			case "TX-1":
				if n.RatingKVA == nil || *n.RatingKVA != 1000 || n.CommissionedDate == nil || *n.CommissionedDate != "2019-04-30" {
					t.Errorf("TX-1 rating/date = %v / %v", n.RatingKVA, n.CommissionedDate)
				}
			case "SWB-1":
				if n.RatingKVA != nil || n.CommissionedDate != nil {
					t.Errorf("SWB-1 has none, got %v / %v", n.RatingKVA, n.CommissionedDate)
				}
			}
		}
	}
	sorted := decode[dto.AssetListResponse](t, get(t, srv, "/api/assets?sort=name&dir=desc"))
	if len(sorted.Items) != 4 || sorted.Items[0].AssetID != "TX-1" || sorted.Items[3].AssetID != "SWP-1" {
		t.Errorf("list sorted by name desc = %+v", sorted.Items)
	}
	if got := get(t, srv, "/api/assets?sort=rating").StatusCode; got != 400 {
		t.Errorf("unknown sort column: status %d, want 400", got)
	}
	if got := get(t, srv, "/api/assets?dir=sideways").StatusCode; got != 400 {
		t.Errorf("unknown sort direction: status %d, want 400", got)
	}
	path := decode[dto.AncestorsResponse](t, get(t, srv, "/api/assets/SWP-1/ancestors"))
	if len(path.Path) != 3 || path.Path[0].AssetID != "SUB-1" {
		t.Errorf("ancestors = %+v", path)
	}
	detail := decode[dto.AssetDetail](t, get(t, srv, "/api/assets/TX-1"))
	if detail.AssetID != "TX-1" || *detail.CommissionedDate != "2019-04-30" {
		t.Errorf("detail = %+v", detail)
	}
	found := decode[dto.SearchResponse](t, get(t, srv, "/api/assets/search?q=switch&type=switchboard"))
	if found.Count != 1 || found.Results[0].AssetID != "SWB-1" {
		t.Errorf("search = %+v", found)
	}
	lookups := decode[dto.LookupsResponse](t, get(t, srv, "/api/lookups"))
	if len(lookups.AssetTypes) != 5 || len(lookups.RootTypes) != 1 || lookups.RootTypes[0] != "SUBSTATION" {
		t.Errorf("lookups = %+v", lookups)
	}
}

func TestAPI_RejectedRowsAreReportedWithTheirLinesAndTheRestIsStored(t *testing.T) {
	srv := newServer(t)
	resp := upload(t, srv, "bad.csv", header+"SUB-1,SUBSTATION,S,IN_SERVICE,,,,,,,\nTX-NR,TRANSFORMER,Neg,IN_SERVICE,SUB-1,,-500,,,,\n")
	imp := decode[dto.ImportResponse](t, resp)
	if resp.StatusCode != 200 || !imp.Committed || imp.ImportedRows != 1 || imp.RejectedRows != 1 || imp.Rejections[0].CSVRow != 3 ||
		imp.Rejections[0].AssetID != "TX-NR" || imp.Rejections[0].Reason != "rating_kva cannot be negative (-500)" {
		t.Fatalf("status %d import %+v", resp.StatusCode, imp)
	}
	roots := decode[dto.RootsResponse](t, get(t, srv, "/api/assets/roots"))
	if roots.Total != 1 || roots.Roots[0].AssetID != "SUB-1" || roots.Roots[0].SubtreeCount != 1 {
		t.Errorf("roots = %+v, want only the valid SUB-1 stored", roots)
	}
}

func TestAPI_ErrorResponses(t *testing.T) {
	srv := newServer(t)
	tests := []struct {
		name   string
		do     func() *http.Response
		status int
	}{
		{"unknown asset", func() *http.Response { return get(t, srv, "/api/assets/NOPE") }, 404},
		{"unknown asset children", func() *http.Response { return get(t, srv, "/api/assets/NOPE/children") }, 404},
		{"unknown asset ancestors", func() *http.Response { return get(t, srv, "/api/assets/NOPE/ancestors") }, 404},
		{"unknown import", func() *http.Response { return get(t, srv, "/api/imports/00000000-0000-0000-0000-000000000000") }, 404},
		{"malformed import id", func() *http.Response { return get(t, srv, "/api/imports/not-a-uuid") }, 400},
		{"search without q", func() *http.Response { return get(t, srv, "/api/assets/search") }, 400},
		{"search unknown type", func() *http.Response { return get(t, srv, "/api/assets/search?q=a&type=PYLON") }, 400},
		{"non-csv upload", func() *http.Response { return upload(t, srv, "grid.xlsx", goodGrid) }, 415},
		{"header missing required columns", func() *http.Response {
			return upload(t, srv, "g.csv", "asset_id,asset_name,notes,extra\nA,B,c,d\n")
		}, 422},
		{"unknown path", func() *http.Response { return get(t, srv, "/api/nope") }, 404},
		{"wrong method on imports", func() *http.Response { return do(t, srv, http.MethodPut, "/api/imports") }, 405},
		{"wrong method on template", func() *http.Response { return do(t, srv, http.MethodPost, "/api/imports/template") }, 405},
		{"wrong method on assets", func() *http.Response {
			resp, _ := http.Post(srv.URL+"/api/assets/roots", "text/plain", strings.NewReader("x"))
			return resp
		}, 405},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			resp := tc.do()
			defer resp.Body.Close()
			if resp.StatusCode != tc.status {
				b, _ := io.ReadAll(resp.Body)
				t.Fatalf("status = %d, want %d; body %s", resp.StatusCode, tc.status, b)
			}
			var e dto.ErrorResponse
			if err := json.NewDecoder(resp.Body).Decode(&e); err != nil || e.Error == "" {
				t.Errorf("error body not an ErrorResponse: %v %+v", err, e)
			}
			if tc.status == 405 && resp.Header.Get("Allow") == "" {
				t.Error("405 without an Allow header")
			}
		})
	}
}

func TestAPI_UploadLargerThanTheLimitIs413(t *testing.T) {
	srv := newServer(t)
	resp := upload(t, srv, "big.csv", header+strings.Repeat("A,B,C,D,E,,,,,,\n", 100000))
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusRequestEntityTooLarge {
		t.Errorf("status = %d, want 413", resp.StatusCode)
	}
}

// TestAPI_GridAssetsFixture runs the real supplied file (222 rows, dates written
// D/M/YY, and about 16 deliberately bad rows) through the whole stack: preview
// first (nothing stored), then a confirmed import of the valid rows.
func TestAPI_GridAssetsFixture(t *testing.T) {
	data, err := os.ReadFile("../../testdata/grid_assets.csv")
	if err != nil {
		t.Skip("backend/testdata/grid_assets.csv not present; skipping fixture test")
	}
	srv := newServer(t)

	prev := decode[dto.ImportPreviewResponse](t, postCSV(t, srv, "/api/imports/preview", "grid_assets.csv", string(data), nil))
	t.Logf("preview: total=%d importable=%d rejected=%d ignored=%v", prev.TotalRows, prev.ImportableRows, prev.RejectedRows, prev.IgnoredColumns)
	if prev.TotalRows != 222 || prev.ImportableRows+prev.RejectedRows != prev.TotalRows || prev.ImportableRows < 190 {
		t.Fatalf("preview = total %d importable %d rejected %d", prev.TotalRows, prev.ImportableRows, prev.RejectedRows)
	}
	if roots := decode[dto.RootsResponse](t, get(t, srv, "/api/assets/roots")); roots.Total != 0 {
		t.Fatalf("a preview stored %d roots", roots.Total)
	}

	reasonFor := map[string]string{}
	for _, r := range prev.Rejections {
		if r.CSVRow < 2 {
			t.Errorf("rejection %+v points at or above the header line", r)
		}
		reasonFor[r.AssetID] = r.Reason
		// The D/M/YY dates are valid: the only row rejected for its date is the
		// deliberately impossible 2026-13-41.
		if strings.Contains(r.Reason, "not a valid date") && r.AssetID != "TX-BD" {
			t.Errorf("%s rejected for its date: %s", r.AssetID, r.Reason)
		}
	}
	// The seeded bad rows, each rejected for its own reason.
	for id, want := range map[string]string{
		"GEN-001": "invalid asset_type",
		"TX-BD":   "not a valid date",
		"TX-NR":   "negative",
		"PNL-BS":  "invalid operational_status",
		"PNL-M":   "does not exist",
		"SWB-S":   "does not exist",
		"SS-H-P":  "must not have a parent",
		"TX-N-P":  "requires a parent",
		"TX-BP":   "must have a parent of type",
		"PNL-BP":  "must have a parent of type",
	} {
		if !strings.Contains(reasonFor[id], want) {
			t.Errorf("%s: reason %q, want it to mention %q", id, reasonFor[id], want)
		}
	}
	for _, id := range []string{"SWB-CY-A", "SWB-CY-B", "SWB-CY-C"} {
		if reasonFor[id] == "" {
			t.Errorf("%s (part of the seeded cycle) was not rejected", id)
		}
	}

	imp := decode[dto.ImportResponse](t, postCSV(t, srv, "/api/imports", "grid_assets.csv", string(data), map[string]string{"expected_fingerprint": prev.Fingerprint}))
	if !imp.Committed || imp.ImportedRows != prev.ImportableRows || imp.RejectedRows != prev.RejectedRows {
		t.Fatalf("import = %+v, want the previewed %d rows stored", imp, prev.ImportableRows)
	}
	stats := decode[dto.StatsResponse](t, get(t, srv, "/api/assets/stats"))
	if stats.Total != imp.ImportedRows {
		t.Errorf("stats total = %d, want %d", stats.Total, imp.ImportedRows)
	}

	// Uploading the same file again stores nothing new: every row is a duplicate
	// or sits under a rejected row, and nothing is ever updated.
	again := decode[dto.ImportResponse](t, upload(t, srv, "grid_assets.csv", string(data)))
	if again.Committed || again.ImportedRows != 0 {
		t.Errorf("re-import = %+v, want nothing stored", again)
	}
}

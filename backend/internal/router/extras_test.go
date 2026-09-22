package router_test

import (
	"bytes"
	"encoding/csv"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"smart-grid-asset-management/backend/internal/entity/dto"
)

// postCSV posts a multipart upload to path with optional extra form fields.
func postCSV(t *testing.T, srv *httptest.Server, path, filename, content string, fields map[string]string) *http.Response {
	t.Helper()
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	for k, v := range fields {
		_ = mw.WriteField(k, v)
	}
	fw, _ := mw.CreateFormFile("file", filename)
	_, _ = io.WriteString(fw, content)
	_ = mw.Close()
	req, _ := http.NewRequest(http.MethodPost, srv.URL+path, &body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set("x-api-key", testKey)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	return resp
}

// do sends a request carrying the API key the test server is configured with.
func do(t *testing.T, srv *httptest.Server, method, path string) *http.Response {
	t.Helper()
	return doWithKey(t, srv, method, path, testKey)
}

func doWithKey(t *testing.T, srv *httptest.Server, method, path, key string) *http.Response {
	t.Helper()
	req, _ := http.NewRequest(method, srv.URL+path, nil)
	if key != "" {
		req.Header.Set("x-api-key", key)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	return resp
}

func status(t *testing.T, srv *httptest.Server, method, path string) int {
	t.Helper()
	resp := do(t, srv, method, path)
	resp.Body.Close()
	return resp.StatusCode
}

// mixed: two valid rows (dates written D/M/YY) and two bad ones.
const mixed = header + `
SUB-1,SUBSTATION,Substation 1,IN_SERVICE,,,,,,,15/2/09
TX-OK,TRANSFORMER,Fine,IN_SERVICE,SUB-1,,1000,,,,12/3/10
TX-NR,TRANSFORMER,Negative,IN_SERVICE,SUB-1,,-500,,,,
TX-KID,TRANSFORMER,Child of bad,IN_SERVICE,TX-NR,,,,,,
`

func TestAPI_PreviewStoresNothingAndConfirmStoresTheValidRows(t *testing.T) {
	srv := newServer(t)

	resp := postCSV(t, srv, "/api/imports/preview", "mixed.csv", mixed, nil)
	if resp.StatusCode != 200 {
		t.Fatalf("preview status %d", resp.StatusCode)
	}
	prev := decode[dto.ImportPreviewResponse](t, resp)
	if prev.TotalRows != 4 || prev.ImportableRows != 2 || prev.RejectedRows != 2 || prev.Fingerprint == "" {
		t.Fatalf("preview = %+v", prev)
	}
	if list := decode[dto.ImportListResponse](t, get(t, srv, "/api/imports")); list.Total != 0 {
		t.Errorf("a preview recorded %d imports, want 0", list.Total)
	}
	if list := decode[dto.AssetListResponse](t, get(t, srv, "/api/assets")); list.Total != 0 {
		t.Errorf("a preview stored %d assets, want 0", list.Total)
	}

	// A wrong fingerprint is refused with 412 and stores nothing.
	stale := postCSV(t, srv, "/api/imports", "mixed.csv", mixed, map[string]string{"expected_fingerprint": "deadbeef"})
	stale.Body.Close()
	if stale.StatusCode != http.StatusPreconditionFailed {
		t.Errorf("stale fingerprint: status %d, want 412", stale.StatusCode)
	}
	if list := decode[dto.AssetListResponse](t, get(t, srv, "/api/assets")); list.Total != 0 {
		t.Errorf("a refused import stored %d assets", list.Total)
	}

	imp := decode[dto.ImportResponse](t, postCSV(t, srv, "/api/imports", "mixed.csv", mixed, map[string]string{"expected_fingerprint": prev.Fingerprint}))
	if !imp.Committed || imp.ImportedRows != 2 || imp.RejectedRows != 2 || len(imp.Rejections) != 2 {
		t.Fatalf("import = %+v", imp)
	}

	// Import activity lists it, and its rejections come from the detail route.
	list := decode[dto.ImportListResponse](t, get(t, srv, "/api/imports"))
	if list.Total != 1 || list.Items[0].ImportID != imp.ImportID || list.Items[0].ImportedRows != 2 || !list.Items[0].Committed {
		t.Errorf("import list = %+v", list)
	}
	if got := decode[dto.ImportResponse](t, get(t, srv, "/api/imports/"+imp.ImportID)); len(got.Rejections) != 2 {
		t.Errorf("detail = %+v", got)
	}
	if code := status(t, srv, http.MethodGet, "/api/imports?limit=1000"); code != 400 {
		t.Errorf("limit above the maximum: status %d, want 400", code)
	}

	// The date written 12/3/10 was stored as 12 March 2010.
	tx := decode[dto.AssetDetail](t, get(t, srv, "/api/assets/TX-OK"))
	if tx.CommissionedDate == nil || *tx.CommissionedDate != "2010-03-12" {
		t.Errorf("TX-OK commissioned %v, want 2010-03-12", tx.CommissionedDate)
	}
}

func TestAPI_TemplateDownloadsAndImports(t *testing.T) {
	srv := newServer(t)
	resp := get(t, srv, "/api/imports/template")
	defer resp.Body.Close()
	if resp.StatusCode != 200 || !strings.HasPrefix(resp.Header.Get("Content-Type"), "text/csv") ||
		!strings.Contains(resp.Header.Get("Content-Disposition"), `attachment; filename="asset_import_template.csv"`) {
		t.Fatalf("status %d, headers %v", resp.StatusCode, resp.Header)
	}
	raw, _ := io.ReadAll(resp.Body)
	records, err := csv.NewReader(bytes.NewReader(raw)).ReadAll()
	if err != nil || len(records) < 2 {
		t.Fatalf("template is not CSV: %v (%d records)", err, len(records))
	}
	imp := decode[dto.ImportResponse](t, upload(t, srv, "asset_import_template.csv", string(raw)))
	if !imp.Committed || imp.RejectedRows != 0 || imp.ImportedRows != len(records)-1 {
		t.Errorf("importing the template: %+v", imp)
	}
}

func TestAPI_ListStatsAndDeletes(t *testing.T) {
	srv := newServer(t)
	upload(t, srv, "grid.csv", goodGrid).Body.Close()

	list := decode[dto.AssetListResponse](t, get(t, srv, "/api/assets?type=transformer&limit=10"))
	if list.Total != 1 || list.Items[0].AssetID != "TX-1" || list.Limit != 10 {
		t.Errorf("list = %+v", list)
	}
	for path, want := range map[string]int{
		"/api/assets?type=PYLON":  400,
		"/api/assets?status=nope": 400,
		"/api/assets?limit=201":   400,
		"/api/assets?offset=-1":   400,
		"/api/assets?limit=x":     400,
	} {
		if got := status(t, srv, http.MethodGet, path); got != want {
			t.Errorf("GET %s = %d, want %d", path, got, want)
		}
	}
	stats := decode[dto.StatsResponse](t, get(t, srv, "/api/assets/stats"))
	if stats.Total != 4 || len(stats.ByType) != 4 {
		t.Errorf("stats = %+v", stats)
	}

	// Delete: 409 with a readable message while children exist, then 204, then 404.
	resp := do(t, srv, http.MethodDelete, "/api/assets/SUB-1")
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("deleting a parent: %d, want 409", resp.StatusCode)
	}
	if msg := decode[dto.ErrorResponse](t, resp).Error; !strings.Contains(msg, "child asset") || !strings.Contains(msg, "SUB-1") {
		t.Errorf("409 message = %q", msg)
	}
	for _, id := range []string{"SWP-1", "SWB-1", "TX-1", "SUB-1"} {
		if got := status(t, srv, http.MethodDelete, "/api/assets/"+id); got != http.StatusNoContent {
			t.Errorf("DELETE %s = %d, want 204", id, got)
		}
	}
	if got := status(t, srv, http.MethodDelete, "/api/assets/SUB-1"); got != http.StatusNotFound {
		t.Errorf("deleting twice: %d, want 404", got)
	}
}

func TestAPI_DeletesNeedTheAPIKey(t *testing.T) {
	srv := newServer(t)
	upload(t, srv, "grid.csv", goodGrid).Body.Close()

	for _, path := range []string{"/api/assets/TX-1"} {
		for name, key := range map[string]string{"no key": "", "wrong key": "nope"} {
			resp := doWithKey(t, srv, http.MethodDelete, path, key)
			resp.Body.Close()
			if resp.StatusCode != http.StatusUnauthorized {
				t.Errorf("DELETE %s with %s = %d, want 401", path, name, resp.StatusCode)
			}
		}
	}
	if got := status(t, srv, http.MethodDelete, "/api/assets/TX-1"); got != http.StatusNoContent {
		t.Fatalf("delete with the right key = %d, want 204", got)
	}
	// The key in the URL is not accepted: it would end up in logs and history.
	resp := doWithKey(t, srv, http.MethodDelete, "/api/assets/SWP-1?api_key="+testKey, "")
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("query-string key alone = %d, want 401", resp.StatusCode)
	}
	// Only TX-1 (deleted with the right header) is gone; the refused calls removed nothing.
	if got := decode[dto.AssetListResponse](t, get(t, srv, "/api/assets")).Total; got != 3 {
		t.Errorf("%d assets left, want 3", got)
	}
}

// Every /api endpoint needs the key; only /healthz and the docs are open.
func TestAPI_EveryEndpointNeedsTheAPIKey(t *testing.T) {
	srv := newServer(t)
	routes := []struct{ method, path string }{
		{http.MethodPost, "/api/imports"},
		{http.MethodPost, "/api/imports/preview"},
		{http.MethodGet, "/api/imports"},
		{http.MethodGet, "/api/imports/template"},
		{http.MethodGet, "/api/imports/00000000-0000-0000-0000-000000000000"},
		{http.MethodGet, "/api/lookups"},
		{http.MethodGet, "/api/assets"},
		{http.MethodGet, "/api/assets/roots"},
		{http.MethodGet, "/api/assets/stats"},
		{http.MethodGet, "/api/assets/search?q=x"},
		{http.MethodGet, "/api/assets/TX-1"},
		{http.MethodDelete, "/api/assets/TX-1"},
		{http.MethodGet, "/api/assets/TX-1/children"},
		{http.MethodGet, "/api/assets/TX-1/ancestors"},
	}
	for _, rt := range routes {
		for name, key := range map[string]string{"no key": "", "wrong key": "nope"} {
			resp := doWithKey(t, srv, rt.method, rt.path, key)
			resp.Body.Close()
			if resp.StatusCode != http.StatusUnauthorized {
				t.Errorf("%s %s with %s = %d, want 401", rt.method, rt.path, name, resp.StatusCode)
			}
		}
		if rt.method != http.MethodGet {
			continue
		}
		resp := do(t, srv, rt.method, rt.path)
		resp.Body.Close()
		if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
			t.Errorf("%s %s with the right key = %d, want it to reach the handler", rt.method, rt.path, resp.StatusCode)
		}
	}

	for _, open := range []string{"/healthz", "/swagger/doc.json", "/swagger/index.html"} {
		resp := doWithKey(t, srv, http.MethodGet, open, "")
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Errorf("GET %s without a key = %d, want 200", open, resp.StatusCode)
		}
	}
}

// With no key configured the API is disabled, not open.
func TestAPI_IsDisabledWithoutAConfiguredKey(t *testing.T) {
	srv := newServerWithKey(t, "")
	for _, key := range []string{"", "anything"} {
		for _, rt := range []struct{ method, path string }{
			{http.MethodGet, "/api/assets/stats"},
			{http.MethodPost, "/api/imports"},
			{http.MethodDelete, "/api/assets/TX-1"},
		} {
			resp := doWithKey(t, srv, rt.method, rt.path, key)
			resp.Body.Close()
			if resp.StatusCode != http.StatusForbidden {
				t.Errorf("%s %s with key %q on a server with no key = %d, want 403", rt.method, rt.path, key, resp.StatusCode)
			}
		}
	}
	resp := doWithKey(t, srv, http.MethodGet, "/healthz", "")
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("/healthz on a server with no key = %d, want 200", resp.StatusCode)
	}
}

// There is no bulk delete: the collection route does not accept DELETE at all.
func TestAPI_ThereIsNoDeleteAll(t *testing.T) {
	srv := newServer(t)
	upload(t, srv, "grid.csv", goodGrid).Body.Close()
	for _, path := range []string{"/api/assets", "/api/assets?confirm=delete-all"} {
		if got := status(t, srv, http.MethodDelete, path); got != http.StatusMethodNotAllowed {
			t.Errorf("DELETE %s = %d, want 405", path, got)
		}
	}
	if got := decode[dto.AssetListResponse](t, get(t, srv, "/api/assets")).Total; got != 4 {
		t.Errorf("%d assets left, want 4", got)
	}
}

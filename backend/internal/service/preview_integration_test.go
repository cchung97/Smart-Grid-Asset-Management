package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	"smart-grid-asset-management/backend/internal/base/csvx"
	"smart-grid-asset-management/backend/internal/base/errx"
	"smart-grid-asset-management/backend/internal/entity/dto"
)

// A file with two good rows and two bad ones (one a child of a bad row).
const mixedGrid = fullHeader + `
SUB-1,SUBSTATION,Substation 1,IN_SERVICE,,,,,,,15/2/09
TX-OK,TRANSFORMER,Fine,IN_SERVICE,SUB-1,,1000,,,,12/3/10
TX-NR,TRANSFORMER,Negative,IN_SERVICE,SUB-1,,-500,,,,
TX-KID,TRANSFORMER,Child of bad,IN_SERVICE,TX-NR,,,,,,
`

func parseGrid(t *testing.T, text string) csvx.ParsedCSV {
	t.Helper()
	parsed, err := csvx.Parse(strings.NewReader(text), "grid.csv", csvx.Options{MinColumns: RequiredColumnCount()})
	if err != nil {
		t.Fatalf("csvx.Parse: %v", err)
	}
	return parsed
}

func (e *env) preview(t *testing.T, text string) dto.ImportPreviewResponse {
	t.Helper()
	resp, err := e.imports.Preview(context.Background(), ImportInput{Filename: "grid.csv", ClientIP: "203.0.113.7", CSV: parseGrid(t, text)})
	if err != nil {
		t.Fatalf("Preview: %v", err)
	}
	return resp
}

func (e *env) confirm(text, fingerprint string, t *testing.T) (dto.ImportResponse, error) {
	t.Helper()
	return e.imports.Import(context.Background(), ImportInput{
		Filename: "grid.csv", ClientIP: "203.0.113.7", CSV: parseGrid(t, text), ExpectedFingerprint: fingerprint,
	})
}

func TestPreview_WritesNothingAndReportsWhatWouldHappen(t *testing.T) {
	e := newEnv(t)
	p := e.preview(t, mixedGrid)
	if p.TotalRows != 4 || p.ImportableRows != 2 || p.RejectedRows != 2 || len(p.Rejections) != 2 || p.Fingerprint == "" {
		t.Fatalf("preview = %+v, want 4 rows: 2 importable, 2 rejected", p)
	}
	if p.Rejections[1].AssetID != "TX-KID" || !strings.Contains(p.Rejections[1].Reason, "ancestor TX-NR was rejected") {
		t.Errorf("rejection = %+v, want TX-KID rejected because its parent was", p.Rejections[1])
	}
	for _, table := range []string{"assets", "import_runs", "import_rejections"} {
		if n := e.count(t, table); n != 0 {
			t.Errorf("%s has %d rows after a preview, want 0", table, n)
		}
	}
	if again := e.preview(t, mixedGrid); again.Fingerprint != p.Fingerprint {
		t.Error("previewing the same file twice gave different fingerprints")
	}
}

func TestImport_WithMatchingFingerprintStoresTheValidRows(t *testing.T) {
	e := newEnv(t)
	p := e.preview(t, mixedGrid)
	res, err := e.confirm(mixedGrid, p.Fingerprint, t)
	if err != nil {
		t.Fatalf("Import: %v", err)
	}
	if !res.Committed || res.ImportedRows != 2 || res.RejectedRows != 2 {
		t.Fatalf("result = %+v", res)
	}
	if e.count(t, "assets") != 2 || e.count(t, "import_runs") != 1 || e.count(t, "import_rejections") != 2 {
		t.Errorf("assets=%d runs=%d rejections=%d, want 2, 1, 2", e.count(t, "assets"), e.count(t, "import_runs"), e.count(t, "import_rejections"))
	}
	// The day-first dates were stored as the right calendar dates.
	var got []string
	if err := e.db.Raw(`SELECT to_char(commissioned_date, 'YYYY-MM-DD') FROM assets WHERE commissioned_date IS NOT NULL ORDER BY asset_id`).Scan(&got).Error; err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0] != "2009-02-15" || got[1] != "2010-03-12" {
		t.Errorf("stored dates = %v, want [2009-02-15 2010-03-12]", got)
	}
}

func TestImport_StaleFingerprintWritesNothing(t *testing.T) {
	e := newEnv(t)
	p := e.preview(t, goodGrid)
	if p.ImportableRows != 6 {
		t.Fatalf("preview = %+v, want all 6 rows importable", p)
	}
	// Someone else imports the same file after the preview was shown.
	if _, err := e.doImport(t, goodGrid); err != nil {
		t.Fatal(err)
	}
	_, err := e.confirm(goodGrid, p.Fingerprint, t)
	if !errors.Is(err, ErrPreviewStale) {
		t.Fatalf("err = %v, want ErrPreviewStale", err)
	}
	if e.count(t, "assets") != 6 || e.count(t, "import_runs") != 1 {
		t.Errorf("assets=%d runs=%d, want the first import untouched and no new audit row", e.count(t, "assets"), e.count(t, "import_runs"))
	}
	// A preview taken now sees the truth, and confirming it is refused only
	// by its own outcome (every row is a duplicate: nothing to store).
	fresh := e.preview(t, goodGrid)
	if fresh.ImportableRows != 0 || fresh.RejectedRows != 6 || fresh.Fingerprint == p.Fingerprint {
		t.Errorf("fresh preview = %+v", fresh)
	}
}

func TestImport_ChangedFileWithSameOutcomeIsStale(t *testing.T) {
	e := newEnv(t)
	p := e.preview(t, mixedGrid)
	edited := strings.Replace(mixedGrid, "Fine", "Edited", 1) // same ids, same rejections, different cell
	if _, err := e.confirm(edited, p.Fingerprint, t); !errors.Is(err, ErrPreviewStale) {
		t.Errorf("err = %v, want ErrPreviewStale: the confirmed file is not the one that was previewed", err)
	}
	if e.count(t, "assets") != 0 {
		t.Error("nothing may be stored")
	}
}

func TestListImports_NewestFirstWithPaging(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	if _, err := e.doImport(t, mixedGrid); err != nil {
		t.Fatal(err)
	}
	if _, err := e.doImport(t, goodGrid); err != nil { // SUB-1 exists: partly rejected
		t.Fatal(err)
	}
	page, err := e.imports.ListImports(ctx, dto.ImportListRequest{})
	if err != nil || page.Total != 2 || len(page.Items) != 2 || page.Limit != 25 {
		t.Fatalf("ListImports() = %+v, %v", page, err)
	}
	if page.Items[0].CreatedAt < page.Items[1].CreatedAt {
		t.Error("newest import must come first")
	}
	if page.Items[0].ClientIP != "203.0.113.7" || page.Items[0].Filename != "grid.csv" {
		t.Errorf("item = %+v", page.Items[0])
	}
	one, err := e.imports.ListImports(ctx, dto.ImportListRequest{Limit: 1, Offset: 1})
	if err != nil || len(one.Items) != 1 || one.Items[0].ImportID != page.Items[1].ImportID || one.Total != 2 {
		t.Errorf("second page = %+v, %v", one, err)
	}
	var ie *errx.InputError
	if _, err := e.imports.ListImports(ctx, dto.ImportListRequest{Limit: 101}); !errors.As(err, &ie) {
		t.Errorf("limit 101: err = %v, want *errx.InputError", err)
	}
}

func TestAssetService_ListStatsAndDeletes(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	if _, err := e.doImport(t, goodGrid); err != nil {
		t.Fatal(err)
	}

	all, err := e.assets.List(ctx, dto.ListRequest{})
	if err != nil || all.Total != 6 || len(all.Items) != 6 || all.Items[0].AssetID != "LV-1" {
		t.Fatalf("List() = %+v, %v (want 6 assets ordered by id)", all, err)
	}
	page, err := e.assets.List(ctx, dto.ListRequest{Limit: 2, Offset: 4})
	if err != nil || page.Total != 6 || len(page.Items) != 2 || page.Items[0].AssetID != "SWP-1" {
		t.Errorf("paged List() = %+v, %v", page, err)
	}
	for name, tc := range map[string]struct {
		req  dto.ListRequest
		want int
	}{
		"by type":         {dto.ListRequest{Type: "transformer"}, 1},
		"by status":       {dto.ListRequest{Status: "in_service"}, 4},
		"by text":         {dto.ListRequest{Q: "substation"}, 2},
		"text is literal": {dto.ListRequest{Q: "%"}, 0},
		"type and status": {dto.ListRequest{Type: "SUBSTATION", Status: "IN_SERVICE"}, 2},
		"no match":        {dto.ListRequest{Q: "zzz"}, 0},
	} {
		got, err := e.assets.List(ctx, tc.req)
		if err != nil || got.Total != tc.want || len(got.Items) != tc.want {
			t.Errorf("%s: total=%d items=%d err=%v, want %d", name, got.Total, len(got.Items), err, tc.want)
		}
	}
	var ie *errx.InputError
	if _, err := e.assets.List(ctx, dto.ListRequest{Type: "PYLON"}); !errors.As(err, &ie) {
		t.Errorf("unknown type: err = %v, want *errx.InputError", err)
	}
	if _, err := e.assets.List(ctx, dto.ListRequest{Status: "BROKEN"}); !errors.As(err, &ie) {
		t.Errorf("unknown status: err = %v, want *errx.InputError", err)
	}

	stats, err := e.assets.Stats(ctx)
	if err != nil || stats.Total != 6 || len(stats.ByType) != 5 || len(stats.ByStatus) != 3 {
		t.Fatalf("Stats() = %+v, %v", stats, err)
	}
	if stats.ByType[0].AssetType != "LV_BOARD" || stats.ByType[0].Count != 1 {
		t.Errorf("by_type[0] = %+v", stats.ByType[0])
	}

	// Delete: refused while it has children, allowed once they are gone.
	var ce *errx.ConflictError
	if err := e.assets.Delete(ctx, "SUB-1"); !errors.As(err, &ce) || !strings.Contains(ce.Msg, "3 child") {
		t.Errorf("deleting a parent: err = %v, want a conflict naming 3 children", err)
	}
	if err := e.assets.Delete(ctx, "NOPE"); !errors.Is(err, errx.ErrNotFound) {
		t.Errorf("deleting an unknown asset: err = %v, want ErrNotFound", err)
	}
	for _, id := range []string{"SWP-1", "SWB-1", "TX-1", "LV-1", "SUB-1"} {
		if err := e.assets.Delete(ctx, id); err != nil {
			t.Fatalf("Delete(%s): %v", id, err)
		}
	}
	if got := e.count(t, "assets"); got != 1 {
		t.Errorf("assets = %d, want only SUB-2 left", got)
	}
}

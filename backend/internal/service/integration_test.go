package service

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"gorm.io/gorm"

	"smart-grid-asset-management/backend/internal/base/csvx"
	"smart-grid-asset-management/backend/internal/base/errx"
	"smart-grid-asset-management/backend/internal/base/logs"
	"smart-grid-asset-management/backend/internal/entity/dto"
	"smart-grid-asset-management/backend/internal/repository"
	"smart-grid-asset-management/backend/internal/testutil"
)

type env struct {
	db      *gorm.DB
	lookups *LookupCache
	imports *ImportService
	assets  *AssetService
}

func newEnv(t *testing.T) *env {
	t.Helper()
	logs.SetOutput(io.Discard)
	t.Cleanup(func() { logs.SetOutput(os.Stdout) })
	db := testutil.NewTestDB(t)
	lookups := NewLookupCache(repository.NewLookupRepository(db))
	if err := lookups.Refresh(context.Background()); err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	return &env{
		db:      db,
		lookups: lookups,
		imports: NewImportService(db, lookups),
		assets:  NewAssetService(repository.NewAssetRepository(db), lookups),
	}
}

func (e *env) count(t *testing.T, table string) int {
	t.Helper()
	var n int
	if err := e.db.Raw("SELECT count(*) FROM " + table).Scan(&n).Error; err != nil {
		t.Fatal(err)
	}
	return n
}

func (e *env) doImport(t *testing.T, csvText string) (importResult, error) {
	t.Helper()
	parsed, err := csvx.Parse(strings.NewReader(csvText), "grid.csv", csvx.Options{MinColumns: RequiredColumnCount()})
	if err != nil {
		t.Fatalf("csvx.Parse: %v", err)
	}
	resp, err := e.imports.Import(context.Background(), ImportInput{Filename: "grid.csv", ClientIP: "203.0.113.7", CSV: parsed})
	return importResult{resp.ImportID, resp.TotalRows, resp.ImportedRows, resp.RejectedRows, resp.Committed, resp.IgnoredColumns, resp.Rejections}, err
}

// A clean hierarchy, deliberately listed children-first.
const goodGrid = fullHeader + `
SWP-1,SWITCHBOARD_PANEL,Panel 1,IN_SERVICE,SWB-1,,,,,,
SWB-1,SWITCHBOARD,Switchboard 1,IN_SERVICE,SUB-1,,,,,,
TX-1,TRANSFORMER,Transformer 1,MAINTENANCE,SUB-1,22,1000,ABB,R1,SN1,2019-04-30
LV-1,LV_BOARD,LV Board 1,OUT_OF_SERVICE,SUB-1,,,,,,
SUB-1,SUBSTATION,Substation 1,IN_SERVICE,,66,,,,,
SUB-2,SUBSTATION,Substation 2,IN_SERVICE,,,,,,,
`

func TestImport_CleanFileCommitsEverythingParentsFirst(t *testing.T) {
	e := newEnv(t)
	res, err := e.doImport(t, goodGrid)
	if err != nil {
		t.Fatalf("Import: %v", err)
	}
	if !res.Committed || res.Total != 6 || res.Imported != 6 || res.Rejected != 0 || len(res.Rejections) != 0 {
		t.Fatalf("result = %+v, want all 6 rows committed", res)
	}
	if got := e.count(t, "assets"); got != 6 {
		t.Errorf("assets = %d, want 6", got)
	}

	var tx1 struct {
		ParentAssetID    *string
		RatingKVA        *float64
		CommissionedDate *time.Time
	}
	if err := e.db.Raw(`SELECT parent_asset_id, rating_kva, commissioned_date FROM assets WHERE asset_id='TX-1'`).Scan(&tx1).Error; err != nil {
		t.Fatal(err)
	}
	if tx1.ParentAssetID == nil || *tx1.ParentAssetID != "SUB-1" || tx1.RatingKVA == nil || *tx1.RatingKVA != 1000 ||
		tx1.CommissionedDate == nil || tx1.CommissionedDate.Format("2006-01-02") != "2019-04-30" {
		t.Errorf("TX-1 stored as %+v", tx1)
	}

	var audit struct {
		Committed bool
		IP        string
	}
	if err := e.db.Raw(`SELECT committed, host(client_ip) AS ip FROM import_runs`).Scan(&audit).Error; err != nil || !audit.Committed || audit.IP != "203.0.113.7" {
		t.Errorf("audit row = %+v err=%v", audit, err)
	}

	got, err := e.imports.GetImport(context.Background(), dto.ImportIDRequest{ImportID: res.ImportID})
	if err != nil || !got.Committed || got.ImportedRows != 6 || got.TotalRows != 6 {
		t.Errorf("GetImport() = %+v, %v", got, err)
	}
}

func TestImport_RejectedRowsAreSkippedAndReportedTheRestAreStored(t *testing.T) {
	e := newEnv(t)
	var logBuf bytes.Buffer
	logs.SetOutput(&logBuf)
	bad := fullHeader + `
SUB-1,SUBSTATION,Substation 1,IN_SERVICE,,,,,,,
TX-OK,TRANSFORMER,Fine,IN_SERVICE,SUB-1,,,,,,
TX-NR,TRANSFORMER,Negative,IN_SERVICE,SUB-1,,-500,,,,
SWB-CY-A,SWITCHBOARD,Cycle A,IN_SERVICE,SWB-CY-B,,,,,,
SWB-CY-B,SWITCHBOARD,Cycle B,IN_SERVICE,SWB-CY-A,,,,,,
TX-KID,TRANSFORMER,Child of bad,IN_SERVICE,TX-NR,,,,,,
`
	res, err := e.doImport(t, bad)
	if err != nil {
		t.Fatalf("Import: %v", err)
	}
	if !res.Committed || res.Imported != 2 || res.Total != 6 || res.Rejected != 4 {
		t.Fatalf("result = %+v, want the 2 valid rows stored and 4 of 6 rows rejected", res)
	}
	if got := e.count(t, "assets"); got != 2 {
		t.Errorf("assets = %d, want 2 (SUB-1 and TX-OK; the rejected rows and TX-KID are skipped)", got)
	}
	if e.count(t, "import_runs") != 1 || e.count(t, "import_rejections") != 4 {
		t.Errorf("audit rows: runs=%d rejections=%d, want 1 and 4", e.count(t, "import_runs"), e.count(t, "import_rejections"))
	}

	want := map[int]string{
		4: "rating_kva cannot be negative (-500)",
		5: "cycle detected: SWB-CY-A -> SWB-CY-B -> SWB-CY-A",
		6: "cycle detected: SWB-CY-B -> SWB-CY-A -> SWB-CY-B",
		7: "ancestor TX-NR was rejected",
	}
	for _, r := range res.Rejections {
		w, ok := want[r.CSVRow]
		if !ok || !strings.HasPrefix(r.Reason, w) {
			t.Errorf("unexpected rejection %+v", r)
		}
		delete(want, r.CSVRow)
	}
	if len(want) != 0 {
		t.Errorf("missing rejections for rows %v", want)
	}

	stored, err := e.imports.GetImport(context.Background(), dto.ImportIDRequest{ImportID: res.ImportID})
	if err != nil || len(stored.Rejections) != 4 || stored.Rejections[0].CSVRow != 4 || stored.Rejections[0].AssetID != "TX-NR" {
		t.Errorf("GetImport() = %+v, %v; want the same rejections re-hydrated in row order", stored, err)
	}

	logged := logBuf.String()
	for _, want := range []string{"import row rejected", "line=4", "asset_id=TX-NR", "pass=1", "line=5", "pass=2", "import finished", "committed=true"} {
		if !strings.Contains(logged, want) {
			t.Errorf("log output missing %q", want)
		}
	}
}

func TestImport_ReimportingACommittedFileRejectsEveryRow(t *testing.T) {
	e := newEnv(t)
	if _, err := e.doImport(t, goodGrid); err != nil {
		t.Fatal(err)
	}
	res, err := e.doImport(t, goodGrid)
	if err != nil {
		t.Fatal(err)
	}
	if res.Committed || res.Rejected != 6 {
		t.Fatalf("result = %+v, want all 6 rows rejected", res)
	}
	if !strings.Contains(res.Rejections[0].Reason, "already exists in the database") {
		t.Errorf("reason = %q", res.Rejections[0].Reason)
	}
	if got := e.count(t, "assets"); got != 6 {
		t.Errorf("assets = %d, want the original 6 untouched", got)
	}
}

func TestImport_SecondFileCanAttachToAssetsAlreadyStored(t *testing.T) {
	e := newEnv(t)
	if _, err := e.doImport(t, goodGrid); err != nil {
		t.Fatal(err)
	}
	res, err := e.doImport(t, fullHeader+"\nTX-2,TRANSFORMER,Second,IN_SERVICE,SUB-2,,,,,,\n")
	if err != nil || !res.Committed {
		t.Fatalf("result = %+v, err = %v; want commit under a stored parent", res, err)
	}
	if got := e.count(t, "assets"); got != 7 {
		t.Errorf("assets = %d, want 7", got)
	}
}

func TestImport_ColumnsAreFlexible(t *testing.T) {
	e := newEnv(t)
	// Reordered, mixed-case headers, an unknown column, most optional
	// columns absent, a UTF-8 BOM, CRLF endings and a trailing blank row.
	text := "\xef\xbb\xbfOperational Status,Asset-Name,Legacy Ref,ASSET_TYPE,asset_id,parent_asset_id\r\n" +
		"IN_SERVICE,Sub,X1,SUBSTATION,SUB-1,\r\n" +
		"IN_SERVICE,Tx,X2,transformer,TX-1,SUB-1\r\n" +
		",,,,,\r\n"
	res, err := e.doImport(t, text)
	if err != nil {
		t.Fatalf("Import: %v", err)
	}
	if !res.Committed || res.Total != 2 {
		t.Fatalf("result = %+v, want 2 rows committed (blank row skipped)", res)
	}
	if len(res.IgnoredColumns) != 1 || res.IgnoredColumns[0] != "Legacy Ref" {
		t.Errorf("IgnoredColumns = %v, want [Legacy Ref]", res.IgnoredColumns)
	}
	var typ string
	if err := e.db.Raw(`SELECT asset_type FROM assets WHERE asset_id='TX-1'`).Scan(&typ).Error; err != nil || typ != "TRANSFORMER" {
		t.Errorf("asset_type = %q, %v; want canonical TRANSFORMER", typ, err)
	}
}

func TestImport_NewTypesAddedToTheDatabaseNeedNoCodeChange(t *testing.T) {
	e := newEnv(t)
	for _, stmt := range []string{
		`INSERT INTO asset_types (code) VALUES ('CABLE')`,
		`INSERT INTO asset_type_parent_rules (child_type_code, parent_type_code) VALUES ('CABLE', 'SUBSTATION')`,
		`INSERT INTO operational_statuses (code) VALUES ('DECOMMISSIONED')`,
	} {
		if err := e.db.Exec(stmt).Error; err != nil {
			t.Fatal(err)
		}
	}
	res, err := e.doImport(t, fullHeader+"\nSUB-1,SUBSTATION,S,IN_SERVICE,,,,,,,\nC-1,CABLE,Feeder,DECOMMISSIONED,SUB-1,,,,,,\n")
	if err != nil || !res.Committed {
		t.Fatalf("result = %+v, err = %v; the new type/status should validate straight from the tables", res, err)
	}
}

func TestImport_StructuralProblems(t *testing.T) {
	e := newEnv(t)

	_, err := e.doImport(t, "asset_id,asset_name,notes,extra\nA,B,c,d\n")
	var se *SchemaError
	if !errors.As(err, &se) || len(se.Missing) != 2 {
		t.Errorf("missing columns: err = %v, want *SchemaError with 2 missing", err)
	}
	if _, err := e.doImport(t, "asset_id,asset_type,asset_name,operational_status\n"); !errors.Is(err, ErrNoDataRows) {
		t.Errorf("header-only file: err = %v, want ErrNoDataRows", err)
	}
	if _, err := e.doImport(t, "asset_id,asset_type,asset_name,operational_status\n,,,\n"); !errors.Is(err, ErrNoDataRows) {
		t.Errorf("only blank rows: err = %v, want ErrNoDataRows", err)
	}
	if got := e.count(t, "import_runs"); got != 0 {
		t.Errorf("structural failures must not create audit rows, got %d", got)
	}
}

func TestImport_ConcurrentIdenticalUploadsNeverDuplicateOrHalfCommit(t *testing.T) {
	e := newEnv(t)
	const workers = 6
	results := make([]importResult, workers)
	errs := make([]error, workers)
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			results[i], errs[i] = e.doImport(t, goodGrid)
		}()
	}
	close(start)
	wg.Wait()

	winners := 0
	for i := range results {
		switch {
		case errs[i] == nil && results[i].Committed:
			winners++
		case errors.Is(errs[i], errx.ErrConflict):
		case errs[i] == nil && !results[i].Committed:
		default:
			t.Errorf("worker %d: unexpected outcome %+v err=%v", i, results[i], errs[i])
		}
	}
	if winners != 1 {
		t.Errorf("committed imports = %d, want exactly 1", winners)
	}
	if got := e.count(t, "assets"); got != 6 {
		t.Errorf("assets = %d, want 6 (no duplicates, no partial commit)", got)
	}
}

func TestGetImport_NotFoundAndBadID(t *testing.T) {
	e := newEnv(t)
	if _, err := e.imports.GetImport(context.Background(), dto.ImportIDRequest{ImportID: "00000000-0000-0000-0000-000000000000"}); !errors.Is(err, errx.ErrNotFound) {
		t.Errorf("unknown id: err = %v, want errx.ErrNotFound", err)
	}
	var ie *errx.InputError
	if _, err := e.imports.GetImport(context.Background(), dto.ImportIDRequest{ImportID: "nope"}); !errors.As(err, &ie) {
		t.Errorf("malformed id: err = %v, want *errx.InputError", err)
	}
}

func TestAssetService_HierarchyEndpoints(t *testing.T) {
	e := newEnv(t)
	if _, err := e.doImport(t, goodGrid); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()

	roots, err := e.assets.Roots(ctx)
	if err != nil || roots.Total != 2 || roots.Roots[0].AssetID != "SUB-1" || roots.Roots[0].SubtreeCount != 5 || roots.Roots[0].ChildCount != 3 || roots.Roots[1].SubtreeCount != 1 {
		t.Errorf("Roots() = %+v, %v", roots, err)
	}

	kids, err := e.assets.Children(ctx, "SUB-1")
	if err != nil {
		t.Fatal(err)
	}
	if kids.TotalChildren != 3 || len(kids.Groups) != 3 {
		t.Fatalf("Children() = %+v", kids)
	}
	if g := kids.Groups[1]; g.AssetType != "SWITCHBOARD" || g.Count != 1 || g.SubtreeCount != 2 || g.Assets[0].ChildCount != 1 {
		t.Errorf("SWITCHBOARD group = %+v", g)
	}
	byType := map[string]int{}
	for _, c := range kids.DescendantCounts {
		byType[c.AssetType] = c.Count
	}
	if byType["SWITCHBOARD_PANEL"] != 1 || byType["TRANSFORMER"] != 1 || len(byType) != 4 {
		t.Errorf("DescendantCounts = %v", kids.DescendantCounts)
	}
	if leaf, err := e.assets.Children(ctx, "SUB-2"); err != nil || leaf.TotalChildren != 0 || leaf.Groups == nil || leaf.DescendantCounts == nil {
		t.Errorf("Children(leaf) = %+v, %v; want empty (non-nil) lists", leaf, err)
	}
	if _, err := e.assets.Children(ctx, "NOPE"); !errors.Is(err, errx.ErrNotFound) {
		t.Errorf("Children(unknown) err = %v, want errx.ErrNotFound", err)
	}

	path, err := e.assets.Ancestors(ctx, "SWP-1")
	if err != nil || len(path.Path) != 3 || path.Path[0].AssetID != "SUB-1" || path.Path[2].AssetID != "SWP-1" {
		t.Errorf("Ancestors() = %+v, %v", path, err)
	}
	if _, err := e.assets.Ancestors(ctx, "NOPE"); !errors.Is(err, errx.ErrNotFound) {
		t.Errorf("Ancestors(unknown) err = %v", err)
	}

	d, err := e.assets.Get(ctx, "TX-1")
	if err != nil || *d.VoltageKV != 22 || *d.CommissionedDate != "2019-04-30" || *d.ParentAssetID != "SUB-1" || d.CreatedAt == "" {
		t.Errorf("Get() = %+v, %v", d, err)
	}
}

func TestAssetService_Search(t *testing.T) {
	e := newEnv(t)
	if _, err := e.doImport(t, goodGrid); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()

	res, err := e.assets.Search(ctx, dto.SearchRequest{Q: "  sub-1 "})
	if err != nil || res.Count != 1 || res.Query != "sub-1" || res.Results[0].AssetID != "SUB-1" {
		t.Errorf("Search(sub-1) = %+v, %v", res, err)
	}
	res, err = e.assets.Search(ctx, dto.SearchRequest{Q: "1", Type: "transformer"})
	if err != nil || res.Count != 1 || res.Type != "TRANSFORMER" {
		t.Errorf("type filter (case-insensitive, echoed canonical) = %+v, %v", res, err)
	}
	res, err = e.assets.Search(ctx, dto.SearchRequest{Q: "1", Limit: 2})
	if err != nil || res.Count != 2 || !res.Truncated {
		t.Errorf("limit=2 over 5 matches = %+v, %v; want 2 results and truncated", res, err)
	}
	if none, err := e.assets.Search(ctx, dto.SearchRequest{Q: "zzz"}); err != nil || none.Results == nil || none.Count != 0 {
		t.Errorf("no match = %+v, %v; want an empty non-nil list", none, err)
	}

	for name, call := range map[string]func() error{
		"empty q": func() error { _, err := e.assets.Search(ctx, dto.SearchRequest{Q: "   "}); return err },
		"long q": func() error {
			_, err := e.assets.Search(ctx, dto.SearchRequest{Q: strings.Repeat("x", 201)})
			return err
		},
		"unknown type": func() error { _, err := e.assets.Search(ctx, dto.SearchRequest{Q: "a", Type: "PYLON"}); return err },
		"limit zero+":  func() error { _, err := e.assets.Search(ctx, dto.SearchRequest{Q: "a", Limit: 201}); return err },
		"limit neg":    func() error { _, err := e.assets.Search(ctx, dto.SearchRequest{Q: "a", Limit: -1}); return err },
	} {
		var ie *errx.InputError
		if err := call(); !errors.As(err, &ie) {
			t.Errorf("%s: err = %v, want *errx.InputError", name, err)
		}
	}
}

type importResult struct {
	ImportID       string
	Total          int
	Imported       int
	Rejected       int
	Committed      bool
	IgnoredColumns []string
	Rejections     []dto.Rejection
}

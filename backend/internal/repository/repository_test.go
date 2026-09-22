package repository_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"smart-grid-asset-management/backend/internal/base/errx"
	"smart-grid-asset-management/backend/internal/entity/model"
	"smart-grid-asset-management/backend/internal/repository"
	"smart-grid-asset-management/backend/internal/testutil"
)

func sp(s string) *string { return &s }

func asset(id string, parent *string, typ, name string) model.Asset {
	return model.Asset{AssetID: id, ParentAssetID: parent, AssetType: typ, AssetName: name, OperationalStatus: "IN_SERVICE"}
}

// seedTree stores:
//
//	SUB-1 ─┬─ TX-1
//	       ├─ LV-1
//	       └─ SWB-1 ─┬─ SWP-1
//	                 └─ SWP-2
//	SUB-2
func seedTree(t *testing.T, repo *repository.AssetRepository) {
	t.Helper()
	err := repo.InsertBatch(context.Background(), []model.Asset{
		asset("SUB-1", nil, "SUBSTATION", "Substation One"),
		asset("SUB-2", nil, "SUBSTATION", "Substation Two"),
		asset("TX-1", sp("SUB-1"), "TRANSFORMER", "Transformer 1"),
		asset("LV-1", sp("SUB-1"), "LV_BOARD", "LV Board 1"),
		asset("SWB-1", sp("SUB-1"), "SWITCHBOARD", "Switchboard 1"),
		asset("SWP-1", sp("SWB-1"), "SWITCHBOARD_PANEL", "Panel 1"),
		asset("SWP-2", sp("SWB-1"), "SWITCHBOARD_PANEL", "Panel 2"),
	})
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
}

func TestLookupRepository_ReadsSeededReferenceData(t *testing.T) {
	db := testutil.NewTestDB(t)
	repo := repository.NewLookupRepository(db)
	ctx := context.Background()

	types, err := repo.AssetTypes(ctx)
	if err != nil || len(types) != 5 || types[0] != "LV_BOARD" {
		t.Errorf("AssetTypes() = %v, %v; want 5 sorted codes", types, err)
	}
	statuses, err := repo.OperationalStatuses(ctx)
	if err != nil || len(statuses) != 3 {
		t.Errorf("OperationalStatuses() = %v, %v; want 3 codes", statuses, err)
	}
	rules, err := repo.ParentRules(ctx)
	if err != nil || len(rules) != 4 {
		t.Fatalf("ParentRules() = %v, %v; want 4 rules", rules, err)
	}
}

func TestAssetRepository_InsertGetAndConflicts(t *testing.T) {
	db := testutil.NewTestDB(t)
	repo := repository.NewAssetRepository(db)
	ctx := context.Background()

	volt, rating := 22.5, 1000.0
	full := asset("SUB-1", nil, "SUBSTATION", "Full")
	full.VoltageKV, full.RatingKVA, full.Manufacturer = &volt, &rating, sp("ABB")
	if err := repo.InsertBatch(ctx, []model.Asset{full}); err != nil {
		t.Fatalf("InsertBatch: %v", err)
	}

	got, err := repo.Get(ctx, "SUB-1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.VoltageKV == nil || *got.VoltageKV != 22.5 || *got.RatingKVA != 1000 || *got.Manufacturer != "ABB" {
		t.Errorf("Get() = %+v, want numeric/text attributes round-tripped", got)
	}
	if got.ParentAssetID != nil || got.Model != nil || got.CommissionedDate != nil {
		t.Errorf("Get() = %+v, want NULL columns as nil", got)
	}

	if _, err := repo.Get(ctx, "NOPE"); !errors.Is(err, errx.ErrNotFound) {
		t.Errorf("Get(missing) error = %v, want ErrNotFound", err)
	}
	if err := repo.InsertBatch(ctx, []model.Asset{full}); !errors.Is(err, errx.ErrConflict) {
		t.Errorf("duplicate insert error = %v, want ErrConflict", err)
	}
	orphan := asset("TX-9", sp("MISSING"), "TRANSFORMER", "Orphan")
	if err := repo.InsertBatch(ctx, []model.Asset{orphan}); !errors.Is(err, errx.ErrConflict) {
		t.Errorf("FK-violating insert error = %v, want ErrConflict", err)
	}
}

func TestAssetRepository_InsertBatchSpansChunks(t *testing.T) {
	db := testutil.NewTestDB(t)
	repo := repository.NewAssetRepository(db)
	ctx := context.Background()

	// 1201 rows => 3 chunks; each row's parent is the previous row, so
	// rows in later chunks reference rows committed by earlier ones.
	assets := []model.Asset{asset("N0", nil, "SUBSTATION", "n0")}
	for i := 1; i <= 1200; i++ {
		assets = append(assets, asset(idOf(i), sp(idOf(i-1)), "TRANSFORMER", "n"))
	}
	if err := repo.InsertBatch(ctx, assets); err != nil {
		t.Fatalf("InsertBatch(1201 chained rows): %v", err)
	}
	path, err := repo.Ancestors(ctx, "N1200")
	if err != nil || len(path) != 65 { // depth cap: asset + 64 ancestors
		t.Errorf("Ancestors depth-cap: len=%d err=%v, want 65", len(path), err)
	}
}

func idOf(i int) string { return "N" + itoa(i) }

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var b []byte
	for ; i > 0; i /= 10 {
		b = append([]byte{byte('0' + i%10)}, b...)
	}
	return string(b)
}

func TestAssetRepository_ExistingByIDs(t *testing.T) {
	db := testutil.NewTestDB(t)
	repo := repository.NewAssetRepository(db)
	seedTree(t, repo)

	got, err := repo.ExistingByIDs(context.Background(), []string{"SUB-1", "TX-1", "NOPE"})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got["SUB-1"] != "SUBSTATION" || got["TX-1"] != "TRANSFORMER" {
		t.Errorf("ExistingByIDs() = %v", got)
	}
	if empty, err := repo.ExistingByIDs(context.Background(), nil); err != nil || len(empty) != 0 {
		t.Errorf("ExistingByIDs(nil) = %v, %v", empty, err)
	}
}

func TestAssetRepository_ListWithCounts(t *testing.T) {
	db := testutil.NewTestDB(t)
	repo := repository.NewAssetRepository(db)
	seedTree(t, repo)
	ctx := context.Background()

	roots, err := repo.ListWithCounts(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	type row struct {
		id             string
		children, size int
	}
	var got []row
	for _, r := range roots {
		got = append(got, row{r.AssetID, r.ChildCount, r.SubtreeCount})
	}
	want := []row{{"SUB-1", 3, 6}, {"SUB-2", 0, 1}}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("roots = %v, want %v", got, want)
	}

	children, err := repo.ListWithCounts(ctx, sp("SUB-1"))
	if err != nil {
		t.Fatal(err)
	}
	got = nil
	for _, r := range children {
		got = append(got, row{r.AssetID, r.ChildCount, r.SubtreeCount})
	}
	// ordered by type: LV_BOARD, SWITCHBOARD, TRANSFORMER
	want = []row{{"LV-1", 0, 1}, {"SWB-1", 2, 3}, {"TX-1", 0, 1}}
	if len(got) != 3 || got[0] != want[0] || got[1] != want[1] || got[2] != want[2] {
		t.Errorf("children = %v, want %v", got, want)
	}

	if none, err := repo.ListWithCounts(ctx, sp("SUB-2")); err != nil || len(none) != 0 {
		t.Errorf("children of leaf = %v, %v; want empty", none, err)
	}
}

func TestAssetRepository_DescendantCounts(t *testing.T) {
	db := testutil.NewTestDB(t)
	repo := repository.NewAssetRepository(db)
	seedTree(t, repo)

	got, err := repo.DescendantCounts(context.Background(), "SUB-1")
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]int{"LV_BOARD": 1, "SWITCHBOARD": 1, "SWITCHBOARD_PANEL": 2, "TRANSFORMER": 1}
	if len(got) != len(want) {
		t.Fatalf("DescendantCounts() = %v, want %v", got, want)
	}
	for _, tc := range got {
		if want[tc.AssetType] != tc.Count {
			t.Errorf("type %s count = %d, want %d", tc.AssetType, tc.Count, want[tc.AssetType])
		}
	}
}

func TestAssetRepository_Ancestors(t *testing.T) {
	db := testutil.NewTestDB(t)
	repo := repository.NewAssetRepository(db)
	seedTree(t, repo)
	ctx := context.Background()

	path, err := repo.Ancestors(ctx, "SWP-1")
	if err != nil {
		t.Fatal(err)
	}
	var ids []string
	for _, a := range path {
		ids = append(ids, a.AssetID)
	}
	if len(ids) != 3 || ids[0] != "SUB-1" || ids[1] != "SWB-1" || ids[2] != "SWP-1" {
		t.Errorf("Ancestors(SWP-1) = %v, want [SUB-1 SWB-1 SWP-1]", ids)
	}
	if top, err := repo.Ancestors(ctx, "SUB-1"); err != nil || len(top) != 1 {
		t.Errorf("Ancestors(root) = %v, %v; want just itself", top, err)
	}
	if _, err := repo.Ancestors(ctx, "NOPE"); !errors.Is(err, errx.ErrNotFound) {
		t.Errorf("Ancestors(missing) error = %v, want ErrNotFound", err)
	}
}

func TestAssetRepository_Search(t *testing.T) {
	db := testutil.NewTestDB(t)
	repo := repository.NewAssetRepository(db)
	seedTree(t, repo)
	ctx := context.Background()
	if err := repo.InsertBatch(ctx, []model.Asset{
		asset("PCT-1", sp("SUB-1"), "TRANSFORMER", "50%_load"),
		asset("SWB-10", sp("SUB-1"), "SWITCHBOARD", "Switchboard Ten"),
	}); err != nil {
		t.Fatal(err)
	}

	ids := func(q, typ string) []string {
		res, err := repo.Search(ctx, q, typ, 50)
		if err != nil {
			t.Fatalf("Search(%q,%q): %v", q, typ, err)
		}
		var out []string
		for _, a := range res {
			out = append(out, a.AssetID)
		}
		return out
	}
	eq := func(got []string, want ...string) bool {
		if len(got) != len(want) {
			return false
		}
		for i := range got {
			if got[i] != want[i] {
				return false
			}
		}
		return true
	}

	if got := ids("swb-1", ""); !eq(got, "SWB-1", "SWB-10") {
		t.Errorf("exact id must rank before longer prefix match: %v", got)
	}
	if got := ids("panel", ""); !eq(got, "SWP-1", "SWP-2") {
		t.Errorf("name match, case-insensitive: %v", got)
	}
	if got := ids("SWITCHBOARD", "SWITCHBOARD_PANEL"); len(got) != 0 {
		t.Errorf("type filter must exclude other types: %v", got)
	}
	if got := ids("%", ""); !eq(got, "PCT-1") {
		t.Errorf("a literal %% must not act as a wildcard: %v", got)
	}
	if got := ids("_", ""); !eq(got, "PCT-1") {
		t.Errorf("a literal _ must match only names containing it: %v", got)
	}
	if res, _ := repo.Search(ctx, "s", "", 2); len(res) != 2 {
		t.Errorf("limit not applied: got %d rows", len(res))
	}
}

func TestImportRepository_RoundTrip(t *testing.T) {
	db := testutil.NewTestDB(t)
	repo := repository.NewImportRepository(db)
	ctx := context.Background()

	run := model.ImportRun{
		ID: uuid.New(), Filename: "grid.csv", ClientIP: "203.0.113.7",
		TotalRows: 3, ImportedRows: 0, RejectedRows: 2, Committed: false,
	}
	if err := repo.CreateRun(ctx, run); err != nil {
		t.Fatalf("CreateRun: %v", err)
	}
	err := repo.CreateRejections(ctx, run.ID, []model.ImportRejection{
		{CSVRowNumber: 9, AssetID: sp("B"), Reason: "later"},
		{CSVRowNumber: 3, AssetID: nil, Reason: "missing required field: asset_id"},
	})
	if err != nil {
		t.Fatalf("CreateRejections: %v", err)
	}

	got, err := repo.GetRun(ctx, run.ID)
	if err != nil || got.Filename != "grid.csv" || got.ClientIP != "203.0.113.7" || got.RejectedRows != 2 || got.Committed {
		t.Fatalf("GetRun() = %+v, %v", got, err)
	}
	rejs, err := repo.ListRejections(ctx, run.ID)
	if err != nil || len(rejs) != 2 {
		t.Fatalf("ListRejections() = %v, %v", rejs, err)
	}
	if rejs[0].CSVRowNumber != 3 || rejs[0].AssetID != nil || rejs[1].CSVRowNumber != 9 || *rejs[1].AssetID != "B" {
		t.Errorf("rejections must be ordered by csv row with NULL asset id preserved: %+v", rejs)
	}
	if _, err := repo.GetRun(ctx, uuid.New()); !errors.Is(err, errx.ErrNotFound) {
		t.Errorf("GetRun(missing) error = %v, want ErrNotFound", err)
	}
}

func TestInTx_RollsBackOnError(t *testing.T) {
	db := testutil.NewTestDB(t)
	ctx := context.Background()
	boom := errors.New("boom")

	err := repository.InTx(ctx, db, func(tx *gorm.DB) error {
		if err := repository.NewAssetRepository(tx).InsertBatch(ctx, []model.Asset{asset("SUB-1", nil, "SUBSTATION", "x")}); err != nil {
			return err
		}
		return boom
	})
	if !errors.Is(err, boom) {
		t.Fatalf("InTx error = %v, want boom", err)
	}
	if ok, _ := repository.NewAssetRepository(db).Exists(ctx, "SUB-1"); ok {
		t.Error("insert survived a rolled-back transaction")
	}
}

func TestModelTableNamesExistInTheSchema(t *testing.T) {
	db := testutil.NewTestDB(t)
	for _, m := range []any{
		&model.Asset{}, &model.AssetType{}, &model.OperationalStatus{},
		&model.ParentRule{}, &model.ImportRun{}, &model.ImportRejection{},
	} {
		if !db.Migrator().HasTable(m) {
			t.Errorf("%T.TableName() names no table in the migrated schema", m)
		}
	}
}

// A big import file references far more ids than Postgres's 65535 bind
// parameters; ExistingByIDs must send them as one array parameter.
func TestAssetRepository_ExistingByIDsBeyondTheParameterLimit(t *testing.T) {
	db := testutil.NewTestDB(t)
	ctx := context.Background()
	repo := repository.NewAssetRepository(db)
	if err := repo.InsertBatch(ctx, []model.Asset{asset("SUB-1", nil, "SUBSTATION", "x")}); err != nil {
		t.Fatal(err)
	}
	ids := make([]string, 70000)
	for i := range ids {
		ids[i] = fmt.Sprintf("NOPE-%d", i)
	}
	ids[69999] = "SUB-1"

	got, err := repo.ExistingByIDs(ctx, ids)
	if err != nil || len(got) != 1 || got["SUB-1"] != "SUBSTATION" {
		t.Fatalf("ExistingByIDs(70000 ids) = %v, %v; want only SUB-1", got, err)
	}
}

func TestAssetRepository_ListSorting(t *testing.T) {
	db := testutil.NewTestDB(t)
	repo := repository.NewAssetRepository(db)
	seedTree(t, repo)
	ctx := context.Background()
	if err := db.Model(&model.Asset{}).Where("asset_id = ?", "TX-1").Update("operational_status", "MAINTENANCE").Error; err != nil {
		t.Fatal(err)
	}

	ids := func(f repository.AssetFilter) []string {
		f.Limit = 50
		got, total, err := repo.List(ctx, f)
		if err != nil || total != 7 {
			t.Fatalf("List(%+v) = %d rows, total %d, err %v", f, len(got), total, err)
		}
		out := make([]string, len(got))
		for i, a := range got {
			out[i] = a.AssetID
		}
		return out
	}

	tests := []struct {
		name string
		f    repository.AssetFilter
		want []string
	}{
		{"default is by asset id", repository.AssetFilter{}, []string{"LV-1", "SUB-1", "SUB-2", "SWB-1", "SWP-1", "SWP-2", "TX-1"}},
		{"asset id descending", repository.AssetFilter{Sort: "asset_id", Desc: true}, []string{"TX-1", "SWP-2", "SWP-1", "SWB-1", "SUB-2", "SUB-1", "LV-1"}},
		{"name is case-insensitive", repository.AssetFilter{Sort: "name"}, []string{"LV-1", "SWP-1", "SWP-2", "SUB-1", "SUB-2", "SWB-1", "TX-1"}},
		{"name descending", repository.AssetFilter{Sort: "name", Desc: true}, []string{"TX-1", "SWB-1", "SUB-2", "SUB-1", "SWP-2", "SWP-1", "LV-1"}},
		{"type, ties by id", repository.AssetFilter{Sort: "type"}, []string{"LV-1", "SUB-1", "SUB-2", "SWB-1", "SWP-1", "SWP-2", "TX-1"}},
		{"status ascending", repository.AssetFilter{Sort: "status"}, []string{"LV-1", "SUB-1", "SUB-2", "SWB-1", "SWP-1", "SWP-2", "TX-1"}},
		{"status descending", repository.AssetFilter{Sort: "status", Desc: true}, []string{"TX-1", "LV-1", "SUB-1", "SUB-2", "SWB-1", "SWP-1", "SWP-2"}},
		{"parent, roots last", repository.AssetFilter{Sort: "parent"}, []string{"LV-1", "SWB-1", "TX-1", "SWP-1", "SWP-2", "SUB-1", "SUB-2"}},
		{"parent descending, roots still last", repository.AssetFilter{Sort: "parent", Desc: true}, []string{"SWP-1", "SWP-2", "LV-1", "SWB-1", "TX-1", "SUB-1", "SUB-2"}},
		{"an unknown key never reaches the query", repository.AssetFilter{Sort: "asset_id; drop table assets"}, []string{"LV-1", "SUB-1", "SUB-2", "SWB-1", "SWP-1", "SWP-2", "TX-1"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := ids(tc.f); fmt.Sprint(got) != fmt.Sprint(tc.want) {
				t.Errorf("order = %v, want %v", got, tc.want)
			}
		})
	}

	// The order holds across pages: page two continues where page one stopped.
	page := func(offset int) []string {
		got, _, err := repo.List(ctx, repository.AssetFilter{Sort: "name", Desc: true, Limit: 3, Offset: offset})
		if err != nil {
			t.Fatal(err)
		}
		out := make([]string, len(got))
		for i, a := range got {
			out[i] = a.AssetID
		}
		return out
	}
	if got, want := fmt.Sprint(page(0), page(3), page(6)), fmt.Sprint([]string{"TX-1", "SWB-1", "SUB-2"}, []string{"SUB-1", "SWP-2", "SWP-1"}, []string{"LV-1"}); got != want {
		t.Errorf("pages = %s, want %s", got, want)
	}
}

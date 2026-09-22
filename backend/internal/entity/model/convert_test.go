package model

import (
	"reflect"
	"testing"
	"time"

	"github.com/google/uuid"

	"smart-grid-asset-management/backend/internal/entity/dto"
)

func strp(s string) *string   { return &s }
func f64p(f float64) *float64 { return &f }
func datep(y int, m time.Month, d int) *time.Time {
	t := time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
	return &t
}

func TestTableNames(t *testing.T) {
	tests := []struct {
		model interface{ TableName() string }
		want  string
	}{
		{Asset{}, "assets"},
		{AssetWithCounts{}, "assets"}, // inherited through the embedded Asset
		{AssetType{}, "asset_types"},
		{OperationalStatus{}, "operational_statuses"},
		{ParentRule{}, "asset_type_parent_rules"},
		{ImportRun{}, "import_runs"},
		{ImportRejection{}, "import_rejections"},
	}
	for _, tc := range tests {
		if got := tc.model.TableName(); got != tc.want {
			t.Errorf("%T.TableName() = %q, want %q", tc.model, got, tc.want)
		}
	}
}

func TestAsset_ToSummaryAndDetail(t *testing.T) {
	created := time.Date(2026, 9, 20, 16, 15, 0, 0, time.FixedZone("SGT", 8*3600))
	a := Asset{
		AssetID: "TX-1", ParentAssetID: strp("SUB-1"), AssetType: "TRANSFORMER", AssetName: "Transformer 1",
		OperationalStatus: "IN_SERVICE", VoltageKV: f64p(22), RatingKVA: f64p(1000),
		Manufacturer: strp("ABB"), Model: strp("R"), SerialNumber: strp("SN-1"),
		CommissionedDate: datep(2019, 4, 30), CreatedAt: created, UpdatedAt: created,
	}

	if got, want := a.ToSummary(), (dto.AssetSummary{
		AssetID: "TX-1", ParentAssetID: strp("SUB-1"), AssetType: "TRANSFORMER", AssetName: "Transformer 1", OperationalStatus: "IN_SERVICE",
	}); !reflect.DeepEqual(got, want) {
		t.Errorf("ToSummary() = %+v, want %+v", got, want)
	}

	d := a.ToDetail()
	if d.CommissionedDate == nil || *d.CommissionedDate != "2019-04-30" {
		t.Errorf("CommissionedDate = %v, want 2019-04-30", d.CommissionedDate)
	}
	if d.CreatedAt != "2026-09-20T08:15:00Z" {
		t.Errorf("CreatedAt = %q, want the UTC RFC 3339 form 2026-09-20T08:15:00Z", d.CreatedAt)
	}
	if d.VoltageKV == nil || *d.VoltageKV != 22 || d.Manufacturer == nil || *d.Manufacturer != "ABB" {
		t.Errorf("optional attributes not carried through: %+v", d)
	}

	bare := Asset{AssetID: "SUB-1", AssetType: "SUBSTATION", AssetName: "Sub", OperationalStatus: "IN_SERVICE"}.ToDetail()
	if bare.CommissionedDate != nil || bare.ParentAssetID != nil || bare.VoltageKV != nil {
		t.Errorf("absent optionals must stay nil: %+v", bare)
	}
}

func TestAssetWithCounts_ToNode(t *testing.T) {
	c := AssetWithCounts{
		Asset:      Asset{AssetID: "SWB-1", AssetType: "SWITCHBOARD", AssetName: "S", OperationalStatus: "IN_SERVICE"},
		ChildCount: 4, SubtreeCount: 5,
	}
	got := c.ToNode()
	if got.AssetID != "SWB-1" || got.ChildCount != 4 || got.SubtreeCount != 5 {
		t.Errorf("ToNode() = %+v", got)
	}
	if got.RatingKVA != nil || got.CommissionedDate != nil {
		t.Errorf("absent rating/date must stay nil: %+v", got)
	}

	rating := 1000.0
	day := time.Date(2019, 4, 30, 0, 0, 0, 0, time.UTC)
	c.RatingKVA, c.CommissionedDate = &rating, &day
	got = c.ToNode()
	if got.RatingKVA == nil || *got.RatingKVA != 1000 || got.CommissionedDate == nil || *got.CommissionedDate != "2019-04-30" {
		t.Errorf("rating/date not carried through: %+v", got)
	}
}

func TestSmallProjections(t *testing.T) {
	if got := (TypeCount{AssetType: "TRANSFORMER", Count: 3}).ToDTO(); got != (dto.TypeCount{AssetType: "TRANSFORMER", Count: 3}) {
		t.Errorf("TypeCount.ToDTO() = %+v", got)
	}
	if got := (ParentRule{ChildType: "TRANSFORMER", ParentType: "SUBSTATION"}).ToDTO(); got != (dto.ParentRule{ChildType: "TRANSFORMER", ParentType: "SUBSTATION"}) {
		t.Errorf("ParentRule.ToDTO() = %+v", got)
	}
}

func TestImportRejection_ToDTO(t *testing.T) {
	tests := []struct {
		name string
		in   ImportRejection
		want dto.Rejection
	}{
		{"with asset id", ImportRejection{CSVRowNumber: 7, AssetID: strp("TX-9"), Reason: "bad"}, dto.Rejection{CSVRow: 7, AssetID: "TX-9", Reason: "bad"}},
		{"row had no asset id", ImportRejection{CSVRowNumber: 8, Reason: "missing required field: asset_id"}, dto.Rejection{CSVRow: 8, Reason: "missing required field: asset_id"}},
	}
	for _, tc := range tests {
		if got := tc.in.ToDTO(); got != tc.want {
			t.Errorf("%s: ToDTO() = %+v, want %+v", tc.name, got, tc.want)
		}
	}
}

func TestImportRun_ToResponse(t *testing.T) {
	id := uuid.New()
	run := ImportRun{ID: id, TotalRows: 10, ImportedRows: 0, RejectedRows: 2, Committed: false}

	got := run.ToResponse([]dto.Rejection{{CSVRow: 3, Reason: "x"}}, []string{"notes"})
	if got.ImportID != id.String() || got.TotalRows != 10 || got.RejectedRows != 2 || got.Committed {
		t.Errorf("ToResponse() = %+v", got)
	}
	if !reflect.DeepEqual(got.IgnoredColumns, []string{"notes"}) || len(got.Rejections) != 1 {
		t.Errorf("rejections/ignored not carried through: %+v", got)
	}

	empty := run.ToResponse(nil, nil)
	if empty.IgnoredColumns == nil || empty.Rejections == nil {
		t.Errorf("nil inputs must render as empty lists, not null: %+v", empty)
	}
}

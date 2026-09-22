package service

import (
	"context"
	"strings"
	"testing"

	"smart-grid-asset-management/backend/internal/base/csvx"
	"smart-grid-asset-management/backend/internal/entity/model"
)

const fullHeader = "asset_id,asset_type,asset_name,operational_status,parent_asset_id,voltage_kv,rating_kva,manufacturer,model,serial_number,commissioned_date"

// testSnapshot mirrors the reference data the migration seeds.
func testSnapshot() *LookupSnapshot {
	return newLookupSnapshot(
		[]string{"SUBSTATION", "TRANSFORMER", "LV_BOARD", "SWITCHBOARD", "SWITCHBOARD_PANEL"},
		[]string{"IN_SERVICE", "MAINTENANCE", "OUT_OF_SERVICE"},
		[]model.ParentRule{
			{ChildType: "TRANSFORMER", ParentType: "SUBSTATION"},
			{ChildType: "LV_BOARD", ParentType: "SUBSTATION"},
			{ChildType: "SWITCHBOARD", ParentType: "SUBSTATION"},
			{ChildType: "SWITCHBOARD_PANEL", ParentType: "SWITCHBOARD"},
		})
}

// parseCSV runs text through the real csvx + column-resolution path.
func parseCSV(t *testing.T, text string) (columnMap, []rawRow) {
	t.Helper()
	parsed, err := csvx.Parse(strings.NewReader(text), "test.csv", csvx.Options{})
	if err != nil {
		t.Fatalf("csvx.Parse: %v", err)
	}
	cm, err := resolveColumns(parsed.Header)
	if err != nil {
		t.Fatalf("resolveColumns: %v", err)
	}
	rows, _ := mapRows(parsed.Rows, cm)
	return cm, rows
}

func pass1(t *testing.T, lines ...string) ([]AcceptedRow, []rowRejection) {
	t.Helper()
	_, rows := parseCSV(t, fullHeader+"\n"+strings.Join(lines, "\n")+"\n")
	acc, rej, err := runPass1(context.Background(), rows, testSnapshot())
	if err != nil {
		t.Fatalf("runPass1: %v", err)
	}
	return acc, rej
}

func strp(s string) *string { return &s }

// acc builds a pass-1 survivor.
func acc(line int, id, typ, parent string) AcceptedRow {
	a := model.Asset{AssetID: id, AssetType: typ, AssetName: id, OperationalStatus: "IN_SERVICE"}
	if parent != "" {
		a.ParentAssetID = strp(parent)
	}
	return AcceptedRow{Line: line, Asset: a}
}

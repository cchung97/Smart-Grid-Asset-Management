package service

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"smart-grid-asset-management/backend/internal/base/csvx"
	"smart-grid-asset-management/backend/internal/defined"
	"smart-grid-asset-management/backend/internal/entity/model"
)

// -------------------------------------------------------------------------
// CSV schema
// -------------------------------------------------------------------------

func TestResolveColumns(t *testing.T) {
	tests := []struct {
		name        string
		header      []string
		wantIgnored []string
		wantAbsent  []string // optional fields expected to be absent from the file
		wantErr     *SchemaError
	}{
		{
			name:   "every column, schema order",
			header: strings.Split(fullHeader, ","),
		},
		{
			name:       "reordered, mixed case and separators, optional columns absent",
			header:     []string{"Asset Name", "ASSET-ID", "operational_status", " Asset_Type "},
			wantAbsent: []string{"parent_asset_id", "voltage_kv", "rating_kva", "manufacturer", "model", "serial_number", "commissioned_date"},
		},
		{
			name:        "unknown columns are ignored, not an error",
			header:      []string{"asset_id", "asset_type", "asset_name", "operational_status", "notes", "Legacy Ref", "notes"},
			wantIgnored: []string{"notes", "Legacy Ref"},
			wantAbsent:  []string{"parent_asset_id", "voltage_kv", "rating_kva", "manufacturer", "model", "serial_number", "commissioned_date"},
		},
		{
			name:       "BOM on the first header and a trailing empty header",
			header:     []string{"\uFEFFasset_id", "asset_type", "asset_name", "operational_status", ""},
			wantAbsent: []string{"parent_asset_id", "voltage_kv", "rating_kva", "manufacturer", "model", "serial_number", "commissioned_date"},
		},
		{
			name:    "missing required columns",
			header:  []string{"asset_id", "asset_name"},
			wantErr: &SchemaError{Missing: []string{"asset_type", "operational_status"}},
		},
		{
			name:    "a known column repeated",
			header:  []string{"asset_id", "asset_type", "asset_name", "operational_status", "Asset ID"},
			wantErr: &SchemaError{Duplicate: []string{"asset_id"}},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cm, err := resolveColumns(tc.header)
			if tc.wantErr != nil {
				var se *SchemaError
				if !errors.As(err, &se) {
					t.Fatalf("error = %v, want *SchemaError", err)
				}
				if !reflect.DeepEqual(se.Missing, tc.wantErr.Missing) || !reflect.DeepEqual(se.Duplicate, tc.wantErr.Duplicate) {
					t.Errorf("SchemaError = missing %v duplicate %v, want missing %v duplicate %v",
						se.Missing, se.Duplicate, tc.wantErr.Missing, tc.wantErr.Duplicate)
				}
				if !strings.Contains(se.Error(), "found columns") {
					t.Errorf("message should list the columns found: %q", se.Error())
				}
				return
			}
			if err != nil {
				t.Fatalf("resolveColumns() error = %v", err)
			}
			if !reflect.DeepEqual(cm.ignored, tc.wantIgnored) {
				t.Errorf("ignored = %v, want %v", cm.ignored, tc.wantIgnored)
			}
			for _, name := range tc.wantAbsent {
				if cm.pos[fieldIndex[name]] != -1 {
					t.Errorf("field %s should be absent, found at column %d", name, cm.pos[fieldIndex[name]])
				}
			}
		})
	}
}

func TestResolveColumns_FindsColumnsByNameNotPosition(t *testing.T) {
	text := "operational_status,asset_name,junk,asset_type,asset_id\nIN_SERVICE,Main Sub,x,SUBSTATION,SUB-1\n"
	cm, rows := parseCSV(t, text)
	if len(rows) != 1 {
		t.Fatalf("rows = %d, want 1", len(rows))
	}
	r := rows[0]
	if r.assetID() != "SUB-1" || r.vals[fieldIndex["asset_name"]] != "Main Sub" || r.vals[fieldIndex["asset_type"]] != "SUBSTATION" {
		t.Errorf("row mapped by position, not header name: %+v", r.vals)
	}
	if !reflect.DeepEqual(cm.ignored, []string{"junk"}) {
		t.Errorf("ignored = %v, want [junk]", cm.ignored)
	}
}

func TestMapRows_SkipsBlankRowsAndTrims(t *testing.T) {
	parsed, err := csvx.Parse(strings.NewReader("asset_id,asset_type,asset_name,operational_status\n,,,\n  A-1 , SUBSTATION ,Name,IN_SERVICE\n \t,,,\n"), "t.csv", csvx.Options{})
	if err != nil {
		t.Fatal(err)
	}
	cm, err := resolveColumns(parsed.Header)
	if err != nil {
		t.Fatal(err)
	}
	rows, blank := mapRows(parsed.Rows, cm)
	if blank != 2 || len(rows) != 1 {
		t.Fatalf("blank=%d rows=%d, want 2 blank and 1 data row", blank, len(rows))
	}
	if rows[0].Line != 3 || rows[0].assetID() != "A-1" || rows[0].vals[fieldIndex["asset_type"]] != "SUBSTATION" {
		t.Errorf("row = %+v, want line 3 with trimmed values", rows[0])
	}
}

func TestRequiredColumnCountIsDerivedFromTheSchema(t *testing.T) {
	if got := RequiredColumnCount(); got != 4 {
		t.Errorf("RequiredColumnCount() = %d, want 4 (asset_id, asset_type, asset_name, operational_status)", got)
	}
}

// --------------------------------------------------------------------------
// Pass 1
// --------------------------------------------------------------------------

func TestRunPass1_RejectionCategories(t *testing.T) {
	const allowedTypes = "LV_BOARD, SUBSTATION, SWITCHBOARD, SWITCHBOARD_PANEL, TRANSFORMER"
	tests := []struct {
		name       string
		row        string
		wantReason string // "" = row must be accepted; otherwise the exact rejection reason
	}{
		{"valid, every column", "TX-1,TRANSFORMER,Tx One,IN_SERVICE,SUB-1,11,1000,ABB,M1,SN1,2020-02-29", ""},
		{"valid, only required values", "SUB-1,SUBSTATION,Sub One,MAINTENANCE,,,,,,,", ""},
		{"type matched case-insensitively", "TX-1,transformer,Tx,in_service,SUB-1,,,,,,", ""},
		{"missing asset_id", ",TRANSFORMER,Tx,IN_SERVICE,SUB-1,,,,,,", "missing required field: asset_id"},
		{"missing asset_type", "TX-1,,Tx,IN_SERVICE,SUB-1,,,,,,", "missing required field: asset_type"},
		{"missing asset_name", "TX-1,TRANSFORMER,,IN_SERVICE,SUB-1,,,,,,", "missing required field: asset_name"},
		{"missing operational_status", "TX-1,TRANSFORMER,Tx,,SUB-1,,,,,,", "missing required field: operational_status"},
		{"bad asset_type", "TX-1,PYLON,Tx,IN_SERVICE,SUB-1,,,,,,", `invalid asset_type "PYLON" (allowed: ` + allowedTypes + `)`},
		{"bad operational_status", "TX-1,TRANSFORMER,Tx,BROKEN,SUB-1,,,,,,", `invalid operational_status "BROKEN" (allowed: IN_SERVICE, MAINTENANCE, OUT_OF_SERVICE)`},
		{"negative rating_kva", "TX-NR,TRANSFORMER,Tx,IN_SERVICE,SUB-1,,-500,,,,", "rating_kva cannot be negative (-500)"},
		{"zero rating_kva is allowed", "TX-Z,TRANSFORMER,Tx,IN_SERVICE,SUB-1,,0,,,,", ""},
		{"non-numeric voltage_kv", "TX-1,TRANSFORMER,Tx,IN_SERVICE,SUB-1,abc,,,,,", `voltage_kv is not a valid number ("abc")`},
		{"non-numeric rating_kva", "TX-1,TRANSFORMER,Tx,IN_SERVICE,SUB-1,,12kva,,,,", `rating_kva is not a valid number ("12kva")`},
		{"NaN is not a number", "TX-1,TRANSFORMER,Tx,IN_SERVICE,SUB-1,NaN,,,,,", `voltage_kv is not a valid number ("NaN")`},
		{"infinity is not a number", "TX-1,TRANSFORMER,Tx,IN_SERVICE,SUB-1,,Inf,,,,", `rating_kva is not a valid number ("Inf")`},
		{"impossible date", "TX-1,TRANSFORMER,Tx,IN_SERVICE,SUB-1,,,,,,2020-13-45", `commissioned_date is not a valid date ("2020-13-45"; expected YYYY-MM-DD or D/M/YY)`},
		{"impossible day/month date", "TX-1,TRANSFORMER,Tx,IN_SERVICE,SUB-1,,,,,,31/2/10", `commissioned_date is not a valid date ("31/2/10"; expected YYYY-MM-DD or D/M/YY)`},
		{"month-first date is not guessed", "TX-1,TRANSFORMER,Tx,IN_SERVICE,SUB-1,,,,,,12/31/2020", `commissioned_date is not a valid date ("12/31/2020"; expected YYYY-MM-DD or D/M/YY)`},
		{
			"every problem on a row is reported, in schema order",
			"TX-1,PYLON,,IN_SERVICE,SUB-1,,-1,,,,nope",
			`invalid asset_type "PYLON" (allowed: ` + allowedTypes + `); missing required field: asset_name; rating_kva cannot be negative (-1); commissioned_date is not a valid date ("nope"; expected YYYY-MM-DD or D/M/YY)`,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			accepted, rejected := pass1(t, tc.row)
			if tc.wantReason == "" {
				if len(accepted) != 1 || len(rejected) != 0 {
					t.Fatalf("accepted=%d rejected=%+v, want the row accepted", len(accepted), rejected)
				}
				return
			}
			if len(accepted) != 0 || len(rejected) != 1 {
				t.Fatalf("accepted=%d rejected=%d, want the row rejected", len(accepted), len(rejected))
			}
			if got := rejected[0]; got.Reason != tc.wantReason || got.Line != 2 || got.Pass != 1 {
				t.Errorf("rejection = %+v\nwant reason %q on line 2", got, tc.wantReason)
			}
		})
	}
}

func TestRunPass1_ParsesValuesIntoTheAsset(t *testing.T) {
	accepted, rejected := pass1(t, "TX-1,transformer,Tx One,maintenance,SUB-1,11.5,1000,ABB,M1,SN1,2020-02-29")
	if len(rejected) != 0 || len(accepted) != 1 {
		t.Fatalf("rejected = %+v", rejected)
	}
	a := accepted[0].Asset
	if a.AssetID != "TX-1" || a.AssetType != "TRANSFORMER" || a.OperationalStatus != "MAINTENANCE" || a.AssetName != "Tx One" {
		t.Errorf("core fields = %+v (lookup values must be stored as canonical codes)", a)
	}
	if a.ParentAssetID == nil || *a.ParentAssetID != "SUB-1" || *a.VoltageKV != 11.5 || *a.RatingKVA != 1000 {
		t.Errorf("parent/numeric fields = %+v", a)
	}
	if *a.Manufacturer != "ABB" || *a.Model != "M1" || *a.SerialNumber != "SN1" {
		t.Errorf("text fields = %+v", a)
	}
	if a.CommissionedDate == nil || a.CommissionedDate.Format("2006-01-02") != "2020-02-29" {
		t.Errorf("commissioned_date = %v, want 2020-02-29", a.CommissionedDate)
	}

	minimal, _ := pass1(t, "SUB-1,SUBSTATION,Sub,IN_SERVICE,,,,,,,")
	m := minimal[0].Asset
	if m.ParentAssetID != nil || m.VoltageKV != nil || m.RatingKVA != nil || m.Manufacturer != nil || m.CommissionedDate != nil {
		t.Errorf("blank optional values must stay nil: %+v", m)
	}
}

func TestRunPass1_DuplicateAssetIDsRejectEveryOccurrence(t *testing.T) {
	accepted, rejected := pass1(t,
		"A-1,SUBSTATION,First,IN_SERVICE,,,,,,,",
		"B-1,SUBSTATION,Unique,IN_SERVICE,,,,,,,",
		"A-1,SUBSTATION,Second,IN_SERVICE,,,,,,,",
		"A-1,SUBSTATION,Third,IN_SERVICE,,,,,,,",
	)
	if len(accepted) != 1 || accepted[0].Asset.AssetID != "B-1" {
		t.Fatalf("accepted = %+v, want only B-1", accepted)
	}
	want := []rowRejection{
		{Line: 2, AssetID: "A-1", Pass: 1, Reason: `duplicate asset_id "A-1" (also on rows 4, 5)`},
		{Line: 4, AssetID: "A-1", Pass: 1, Reason: `duplicate asset_id "A-1" (also on rows 2, 5)`},
		{Line: 5, AssetID: "A-1", Pass: 1, Reason: `duplicate asset_id "A-1" (also on rows 2, 4)`},
	}
	if !reflect.DeepEqual(rejected, want) {
		t.Errorf("rejected = %+v\nwant     %+v", rejected, want)
	}
}

func TestRunPass1_ReportsOriginalLineNumbers(t *testing.T) {
	// The quoted name spans two physical lines, so the next row is line 4.
	_, rows := parseCSV(t, fullHeader+"\nA-1,SUBSTATION,\"Two\nLines\",IN_SERVICE,,,,,,,\nB-1,PYLON,B,IN_SERVICE,,,,,,,\n")
	_, rejected, err := runPass1(context.Background(), rows, testSnapshot())
	if err != nil || len(rejected) != 1 || rejected[0].Line != 4 {
		t.Fatalf("rejected = %+v, err = %v; want one rejection on line 4", rejected, err)
	}
}

// Parallel fan-out must be indistinguishable from a plain loop: same
// results, same order. Run under -race this also proves the workers do not
// share writable state.
func TestRunPass1_ParallelMatchesSequential(t *testing.T) {
	var sb strings.Builder
	sb.WriteString(fullHeader + "\n")
	const n = 5000 // ~10 chunks, so the worker pool really is used
	for i := 0; i < n; i++ {
		switch {
		case i%97 == 0:
			fmt.Fprintf(&sb, "ID-%d,PYLON,Bad type,IN_SERVICE,,,,,,,\n", i)
		case i%89 == 0:
			fmt.Fprintf(&sb, "ID-%d,TRANSFORMER,Neg,IN_SERVICE,SUB,,-%d,,,,\n", i, i)
		case i%83 == 0:
			fmt.Fprintf(&sb, "ID-%d,TRANSFORMER,Dup,IN_SERVICE,SUB,,,,,,\nID-%d,TRANSFORMER,Dup,IN_SERVICE,SUB,,,,,,\n", i, i)
		default:
			fmt.Fprintf(&sb, "ID-%d,TRANSFORMER,Fine,IN_SERVICE,SUB,%d,%d,,,,2021-01-01\n", i, i, i)
		}
	}
	_, rows := parseCSV(t, sb.String())
	snap := testSnapshot()

	gotAcc, gotRej, err := runPass1(context.Background(), rows, snap)
	if err != nil {
		t.Fatal(err)
	}

	dups := duplicateIDs(rows)
	var wantAcc []AcceptedRow
	var wantRej []rowRejection
	for _, row := range rows {
		res := validateRow(row, snap, dups)
		if len(res.problems) == 0 {
			wantAcc = append(wantAcc, AcceptedRow{Line: row.Line, Asset: res.asset})
		} else {
			wantRej = append(wantRej, rowRejection{Line: row.Line, AssetID: row.assetID(), Reason: strings.Join(res.problems, "; "), Pass: 1})
		}
	}
	if !reflect.DeepEqual(gotAcc, wantAcc) || !reflect.DeepEqual(gotRej, wantRej) {
		t.Fatalf("parallel result differs from sequential (accepted %d vs %d, rejected %d vs %d)",
			len(gotAcc), len(wantAcc), len(gotRej), len(wantRej))
	}
	if len(gotRej) == 0 || len(gotAcc) == 0 {
		t.Fatal("test data should produce both accepted and rejected rows")
	}
}

func TestRunPass1_StopsWhenContextIsCancelled(t *testing.T) {
	_, rows := parseCSV(t, fullHeader+"\nA,SUBSTATION,A,IN_SERVICE,,,,,,,\n")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, _, err := runPass1(ctx, rows, testSnapshot()); err == nil {
		t.Error("runPass1 with a cancelled context returned nil error")
	}
}

// --------------------------------------------------------------------------
// Pass 2
// --------------------------------------------------------------------------

func TestRunPass2(t *testing.T) {
	tests := []struct {
		name          string
		rows          []AcceptedRow
		existing      map[string]string
		pass1Rejected map[string]bool
		// want maps asset_id -> exact rejection reason; ids absent from it must be accepted.
		want map[string]string
	}{
		{
			name: "valid two-level hierarchy",
			rows: []AcceptedRow{acc(2, "SUB-1", "SUBSTATION", ""), acc(3, "TX-1", "TRANSFORMER", "SUB-1"), acc(4, "SWB-1", "SWITCHBOARD", "SUB-1"), acc(5, "SWP-1", "SWITCHBOARD_PANEL", "SWB-1")},
			want: map[string]string{},
		},
		{
			name: "children listed before their parents (row order must not matter)",
			rows: []AcceptedRow{acc(2, "SWP-1", "SWITCHBOARD_PANEL", "SWB-1"), acc(3, "SWB-1", "SWITCHBOARD", "SUB-1"), acc(4, "SUB-1", "SUBSTATION", "")},
			want: map[string]string{},
		},
		{
			name:     "parent already in the database",
			rows:     []AcceptedRow{acc(2, "TX-2", "TRANSFORMER", "SUB-DB")},
			existing: map[string]string{"SUB-DB": "SUBSTATION"},
			want:     map[string]string{},
		},
		{
			name: "substation with a parent",
			rows: []AcceptedRow{acc(2, "SUB-1", "SUBSTATION", "SUB-0"), acc(3, "SUB-0", "SUBSTATION", "")},
			want: map[string]string{"SUB-1": `SUBSTATION is a root type and must not have a parent_asset_id (got "SUB-0")`},
		},
		{
			name: "non-root without a parent (orphan)",
			rows: []AcceptedRow{acc(2, "TX-1", "TRANSFORMER", "")},
			want: map[string]string{"TX-1": "TRANSFORMER requires a parent_asset_id"},
		},
		{
			name: "parent exists nowhere",
			rows: []AcceptedRow{acc(2, "TX-1", "TRANSFORMER", "GHOST")},
			want: map[string]string{"TX-1": `parent "GHOST" does not exist in the database or file`},
		},
		{
			name: "asset is its own parent",
			rows: []AcceptedRow{acc(2, "TX-1", "TRANSFORMER", "TX-1")},
			want: map[string]string{"TX-1": "asset cannot be its own parent"},
		},
		{
			name: "wrong parent type (in file)",
			rows: []AcceptedRow{acc(2, "SUB-1", "SUBSTATION", ""), acc(3, "TX-1", "TRANSFORMER", "SUB-1"), acc(4, "SWP-1", "SWITCHBOARD_PANEL", "TX-1")},
			want: map[string]string{"SWP-1": `parent "TX-1" is a TRANSFORMER, but a SWITCHBOARD_PANEL must have a parent of type SWITCHBOARD`},
		},
		{
			name:     "wrong parent type (in database)",
			rows:     []AcceptedRow{acc(2, "TX-1", "TRANSFORMER", "SWB-DB")},
			existing: map[string]string{"SWB-DB": "SWITCHBOARD"},
			want:     map[string]string{"TX-1": `parent "SWB-DB" is a SWITCHBOARD, but a TRANSFORMER must have a parent of type SUBSTATION`},
		},
		{
			name:     "asset_id already stored in the database",
			rows:     []AcceptedRow{acc(2, "SUB-1", "SUBSTATION", "")},
			existing: map[string]string{"SUB-1": "SUBSTATION"},
			want:     map[string]string{"SUB-1": `asset_id "SUB-1" already exists in the database`},
		},
		{
			name: "two-node cycle",
			rows: []AcceptedRow{acc(2, "SWB-CY-A", "SWITCHBOARD", "SWB-CY-B"), acc(3, "SWB-CY-B", "SWITCHBOARD", "SWB-CY-A")},
			want: map[string]string{
				"SWB-CY-A": `cycle detected: SWB-CY-A -> SWB-CY-B -> SWB-CY-A; parent "SWB-CY-B" is a SWITCHBOARD, but a SWITCHBOARD must have a parent of type SUBSTATION`,
				"SWB-CY-B": `cycle detected: SWB-CY-B -> SWB-CY-A -> SWB-CY-B; parent "SWB-CY-A" is a SWITCHBOARD, but a SWITCHBOARD must have a parent of type SUBSTATION`,
			},
		},
		{
			name: "three-node cycle, with a healthy neighbour left alone",
			rows: []AcceptedRow{
				acc(2, "SUB-1", "SUBSTATION", ""), acc(3, "TX-1", "TRANSFORMER", "SUB-1"),
				acc(4, "A", "SWITCHBOARD", "C"), acc(5, "B", "SWITCHBOARD", "A"), acc(6, "C", "SWITCHBOARD", "B"),
			},
			want: map[string]string{
				"A": `cycle detected: A -> C -> B -> A; parent "C" is a SWITCHBOARD, but a SWITCHBOARD must have a parent of type SUBSTATION`,
				"B": `cycle detected: B -> A -> C -> B; parent "A" is a SWITCHBOARD, but a SWITCHBOARD must have a parent of type SUBSTATION`,
				"C": `cycle detected: C -> B -> A -> C; parent "B" is a SWITCHBOARD, but a SWITCHBOARD must have a parent of type SUBSTATION`,
			},
		},
		{
			name: "a child of a cycle member is rejected as a descendant",
			rows: []AcceptedRow{acc(2, "A", "SWITCHBOARD", "B"), acc(3, "B", "SWITCHBOARD", "A"), acc(4, "P1", "SWITCHBOARD_PANEL", "A")},
			want: map[string]string{
				"A":  `cycle detected: A -> B -> A; parent "B" is a SWITCHBOARD, but a SWITCHBOARD must have a parent of type SUBSTATION`,
				"B":  `cycle detected: B -> A -> B; parent "A" is a SWITCHBOARD, but a SWITCHBOARD must have a parent of type SUBSTATION`,
				"P1": "ancestor A was rejected",
			},
		},
		{
			name: "transitive cascade names the root cause (children after parents)",
			rows: []AcceptedRow{acc(2, "SUB-1", "SUBSTATION", "X"), acc(3, "SWB-1", "SWITCHBOARD", "SUB-1"), acc(4, "SWP-1", "SWITCHBOARD_PANEL", "SWB-1")},
			want: map[string]string{
				"SUB-1": `SUBSTATION is a root type and must not have a parent_asset_id (got "X")`,
				"SWB-1": "ancestor SUB-1 was rejected",
				"SWP-1": "ancestor SUB-1 was rejected",
			},
		},
		{
			name: "transitive cascade names the same root cause (children before parents)",
			rows: []AcceptedRow{acc(2, "SWP-1", "SWITCHBOARD_PANEL", "SWB-1"), acc(3, "SWB-1", "SWITCHBOARD", "SUB-1"), acc(4, "SUB-1", "SUBSTATION", "X")},
			want: map[string]string{
				"SUB-1": `SUBSTATION is a root type and must not have a parent_asset_id (got "X")`,
				"SWB-1": "ancestor SUB-1 was rejected",
				"SWP-1": "ancestor SUB-1 was rejected",
			},
		},
		{
			name: "cascade: child of a row rejected in its own right, including transitively",
			rows: []AcceptedRow{
				acc(2, "SUB-1", "SUBSTATION", ""),
				acc(3, "SWB-1", "SWITCHBOARD", "GHOST"), // parent missing
				acc(4, "SWP-1", "SWITCHBOARD_PANEL", "SWB-1"),
			},
			want: map[string]string{
				"SWB-1": `parent "GHOST" does not exist in the database or file`,
				"SWP-1": "ancestor SWB-1 was rejected",
			},
		},
		{
			name:          "cascade: parent was rejected in pass 1",
			rows:          []AcceptedRow{acc(3, "TX-1", "TRANSFORMER", "SUB-BAD")},
			pass1Rejected: map[string]bool{"SUB-BAD": true},
			want:          map[string]string{"TX-1": "ancestor SUB-BAD was rejected"},
		},
		{
			name:     "asset_id also in the database: a child attaches to the stored asset",
			rows:     []AcceptedRow{acc(2, "SUB-1", "SUBSTATION", ""), acc(3, "TX-1", "TRANSFORMER", "SUB-1")},
			existing: map[string]string{"SUB-1": "SUBSTATION"},
			want:     map[string]string{"SUB-1": `asset_id "SUB-1" already exists in the database`},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			acceptedRows, rejected := runPass2(pass2Input{
				Accepted: tc.rows, Existing: tc.existing, Pass1Rejected: tc.pass1Rejected, Snap: testSnapshot(),
			})
			got := map[string]string{}
			for _, r := range rejected {
				got[r.AssetID] = r.Reason
				if r.Pass != 2 {
					t.Errorf("%s: Pass = %d, want 2", r.AssetID, r.Pass)
				}
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("rejections differ\n got %v\nwant %v", got, tc.want)
			}
			if len(acceptedRows)+len(rejected) != len(tc.rows) {
				t.Errorf("accepted %d + rejected %d != %d input rows", len(acceptedRows), len(rejected), len(tc.rows))
			}
			for _, a := range acceptedRows {
				if _, bad := tc.want[a.Asset.AssetID]; bad {
					t.Errorf("%s was both rejected and accepted", a.Asset.AssetID)
				}
			}
		})
	}
}

func TestRunPass2_KeepsFileOrderAndOriginalLines(t *testing.T) {
	accepted, rejected := runPass2(pass2Input{
		Accepted: []AcceptedRow{acc(7, "TX-1", "TRANSFORMER", "SUB-1"), acc(9, "TX-X", "TRANSFORMER", ""), acc(12, "SUB-1", "SUBSTATION", "")},
		Snap:     testSnapshot(),
	})
	if len(accepted) != 2 || accepted[0].Line != 7 || accepted[1].Line != 12 {
		t.Errorf("accepted lines = %+v, want 7 then 12", accepted)
	}
	if len(rejected) != 1 || rejected[0].Line != 9 {
		t.Errorf("rejected = %+v, want the row on line 9", rejected)
	}
}

func TestRunPass2_LongChainDoesNotRecurseUnboundedly(t *testing.T) {
	// A 20k-deep valid-looking chain whose root is missing exercises the
	// cascade walk; it must finish and reject every row.
	rows := make([]AcceptedRow, 0, 20000)
	rows = append(rows, acc(2, "N0", "TRANSFORMER", "GHOST"))
	for i := 1; i < 20000; i++ {
		rows = append(rows, acc(i+2, idOf(i), "SWITCHBOARD_PANEL", idOf(i-1)))
	}
	_, rejected := runPass2(pass2Input{Accepted: rows, Snap: testSnapshot()})
	if len(rejected) != len(rows) {
		t.Errorf("rejected %d of %d rows", len(rejected), len(rows))
	}
}

func idOf(i int) string { return "N" + string(rune('0'+i%10)) + "-" + itoa(i) }

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

func TestOrderParentsFirst(t *testing.T) {
	rows := []AcceptedRow{
		acc(2, "SWP-1", "SWITCHBOARD_PANEL", "SWB-1"),
		acc(3, "TX-DB", "TRANSFORMER", "SUB-STORED"), // parent already in the database: depth 0
		acc(4, "SWB-1", "SWITCHBOARD", "SUB-1"),
		acc(5, "SUB-1", "SUBSTATION", ""),
		acc(6, "SWP-2", "SWITCHBOARD_PANEL", "SWB-1"),
	}
	got := orderParentsFirst(rows)

	pos := map[string]int{}
	for i, a := range got {
		pos[a.AssetID] = i
	}
	if len(got) != len(rows) {
		t.Fatalf("len = %d, want %d", len(got), len(rows))
	}
	for _, r := range rows {
		if p := r.Asset.ParentAssetID; p != nil {
			if pp, inFile := pos[*p]; inFile && pp > pos[r.Asset.AssetID] {
				t.Errorf("%s is inserted before its parent %s", r.Asset.AssetID, *p)
			}
		}
	}
	// Within a depth the file order is kept: TX-DB (line 3) and SUB-1 (line 5) are depth 0.
	if got[0].AssetID != "TX-DB" || got[1].AssetID != "SUB-1" {
		t.Errorf("depth-0 rows = %s, %s; want file order TX-DB, SUB-1", got[0].AssetID, got[1].AssetID)
	}
	if got[len(got)-2].AssetID != "SWP-1" || got[len(got)-1].AssetID != "SWP-2" {
		t.Errorf("deepest rows should keep file order: %v", got)
	}
}

// TestDateField pins the accepted commissioned_date formats. The supplied data
// is day-first D/M/YY, so those must load; month-first and impossible dates
// must not.
func TestDateField(t *testing.T) {
	spec := dateField("commissioned_date", func(a *model.Asset, tm time.Time) { a.CommissionedDate = &tm })
	tests := []struct {
		raw  string
		want string // YYYY-MM-DD, or "" when the value must be rejected
	}{
		{"2019-03-14", "2019-03-14"},
		{"15/2/09", "2009-02-15"},
		{"12/3/10", "2010-03-12"}, // day first: 12 March, not 3 December
		{"20/6/11", "2011-06-20"},
		{"13/9/10", "2010-09-13"},
		{"05/02/2009", "2009-02-05"},
		{"5/2/2009", "2009-02-05"},
		{"1/1/68", "2068-01-01"}, // Go's pivot: 00-68 -> 20xx
		{"1/1/69", "1969-01-01"}, // 69-99 -> 19xx
		{"29/2/20", "2020-02-29"},
		{"2026-13-41", ""},
		{"31/2/10", ""},
		{"29/2/21", ""}, // 2021 is not a leap year
		{"12/31/2020", ""},
		{"2019-3-4", ""},
		{"15-2-09", ""},
		{"tomorrow", ""},
	}
	for _, tt := range tests {
		t.Run(tt.raw, func(t *testing.T) {
			set, problem := spec.check(tt.raw, nil)
			if tt.want == "" {
				if set != nil || problem == "" {
					t.Fatalf("%q accepted, want rejected", tt.raw)
				}
				return
			}
			if problem != "" {
				t.Fatalf("%q rejected: %s", tt.raw, problem)
			}
			var a model.Asset
			set(&a)
			if got := a.CommissionedDate.Format(defined.DateFormat); got != tt.want {
				t.Errorf("%q parsed as %s, want %s", tt.raw, got, tt.want)
			}
		})
	}
}

// TestTemplateImportsCleanly guarantees the downloadable template can never
// drift from the importer: it parses as CSV, resolves every column, and every
// example row validates.
func TestTemplateImportsCleanly(t *testing.T) {
	svc := &ImportService{}
	text := string(svc.Template())
	if !strings.HasPrefix(text, "asset_id,") {
		t.Fatalf("template starts %q", text[:40])
	}
	cm, rows := parseCSV(t, text)
	if len(cm.ignored) != 0 || len(rows) != len(templateExamples) {
		t.Fatalf("ignored columns %v, %d rows (want 0 and %d)", cm.ignored, len(rows), len(templateExamples))
	}
	snap := testSnapshot()
	survivors, rej1, err := runPass1(context.Background(), rows, snap)
	if err != nil || len(rej1) != 0 {
		t.Fatalf("pass 1: %v, rejections %+v", err, rej1)
	}
	valid, rej2 := runPass2(pass2Input{Accepted: survivors, Pass1Rejected: map[string]bool{}, Existing: map[string]string{}, Snap: snap})
	if len(rej2) != 0 || len(valid) != len(templateExamples) {
		t.Fatalf("pass 2: %d valid, rejections %+v", len(valid), rej2)
	}
}

func TestFingerprint(t *testing.T) {
	csvA := csvx.ParsedCSV{Header: []string{"asset_id"}, Rows: []csvx.Row{{Line: 2, Fields: []string{"A"}}}}
	csvB := csvx.ParsedCSV{Header: []string{"asset_id"}, Rows: []csvx.Row{{Line: 2, Fields: []string{"B"}}}}
	valid := []AcceptedRow{acc(2, "A", "SUBSTATION", "")}
	rej := []rowRejection{{Line: 3, AssetID: "X", Reason: "bad"}}

	base := fingerprint(csvA, valid, rej)
	if base != fingerprint(csvA, valid, rej) {
		t.Error("same input gave two fingerprints")
	}
	for name, other := range map[string]string{
		"different cell":       fingerprint(csvB, valid, rej),
		"different rejection":  fingerprint(csvA, valid, []rowRejection{{Line: 3, AssetID: "X", Reason: "worse"}}),
		"no rejection":         fingerprint(csvA, valid, nil),
		"different valid rows": fingerprint(csvA, []AcceptedRow{acc(2, "Z", "SUBSTATION", "")}, rej),
	} {
		if other == base {
			t.Errorf("%s: fingerprint did not change", name)
		}
	}
	// Length prefixes: moving a character across a field boundary must not collide.
	if fingerprint(csvx.ParsedCSV{Header: []string{"ab", "c"}}, nil, nil) == fingerprint(csvx.ParsedCSV{Header: []string{"a", "bc"}}, nil, nil) {
		t.Error("field boundaries are ambiguous")
	}
}

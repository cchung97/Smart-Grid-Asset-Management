package service

import (
	"bytes"
	"cmp"
	"context"
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"golang.org/x/sync/errgroup"
	"gorm.io/gorm"

	"smart-grid-asset-management/backend/internal/base/csvx"
	"smart-grid-asset-management/backend/internal/base/logs"
	"smart-grid-asset-management/backend/internal/base/parallel"
	"smart-grid-asset-management/backend/internal/defined"
	"smart-grid-asset-management/backend/internal/entity/dto"
	"smart-grid-asset-management/backend/internal/entity/model"
	"smart-grid-asset-management/backend/internal/repository"
)

// ErrNoDataRows means an uploaded CSV has a header but nothing to import.
var ErrNoDataRows = errors.New("file has a header but no data rows")

// ErrPreviewStale means the file, or the stored data it is checked against,
// changed after the preview the caller confirmed: what would be written is no
// longer what they approved. Nothing was written; they should preview again.
var ErrPreviewStale = errors.New("the file's validation result changed since the preview; review it again")

// ImportService validates an uploaded asset CSV in two passes and commits the
// rows that pass (docs/ARCHITECTURE.md section 4). Preview runs the same
// validation without writing anything, so a user can review the outcome
// before confirming.
type ImportService struct {
	db      *gorm.DB
	assets  *repository.AssetRepository
	imports *repository.ImportRepository
	lookups *LookupCache
}

func NewImportService(db *gorm.DB, lookups *LookupCache) *ImportService {
	return &ImportService{
		db:      db,
		assets:  repository.NewAssetRepository(db),
		imports: repository.NewImportRepository(db),
		lookups: lookups,
	}
}

// ImportInput is a structurally valid CSV plus the request facts an
// import is audited with.
type ImportInput struct {
	Filename string
	ClientIP string
	CSV      csvx.ParsedCSV
	// ExpectedFingerprint, when set, is the fingerprint the caller saw in a
	// preview of this file. Import refuses (ErrPreviewStale) if validating
	// again yields a different one.
	ExpectedFingerprint string
}

// validation is the complete, read-only outcome of validating one file.
type validation struct {
	cols        columnMap
	totalRows   int
	blankRows   int
	valid       []AcceptedRow
	rejections  []rowRejection
	fingerprint string
}

// validate runs both passes over the file against the current stored data.
// It never writes. Errors are *SchemaError, ErrNoDataRows, or an unexpected
// wrapped error; per-row problems are part of the result, not errors.
func (s *ImportService) validate(ctx context.Context, in ImportInput) (validation, error) {
	log := logs.WithCtx(ctx)

	cols, err := resolveColumns(in.CSV.Header)
	if err != nil {
		log.Warn("import refused: header does not match the asset schema", "file", in.Filename, "line", in.CSV.HeaderLine, "err", err)
		return validation{}, err
	}
	rows, blank := mapRows(in.CSV.Rows, cols)
	if len(rows) == 0 {
		log.Warn("import refused: no data rows", "file", in.Filename, "blank_rows", blank)
		return validation{}, ErrNoDataRows
	}

	// Two independent reads, overlapped: the reference tables, and which
	// of the file's ids already exist in the database.
	var existing map[string]string
	g, gctx := errgroup.WithContext(ctx)
	g.Go(func() error { return s.lookups.Refresh(gctx) })
	g.Go(func() (err error) {
		existing, err = s.assets.ExistingByIDs(gctx, referencedIDs(rows))
		return err
	})
	if err := g.Wait(); err != nil {
		return validation{}, fmt.Errorf("load validation context: %w", err)
	}
	snap := s.lookups.Snapshot() // one snapshot for the whole run

	survivors, rej1, err := runPass1(ctx, rows, snap)
	if err != nil {
		return validation{}, fmt.Errorf("field validation: %w", err)
	}
	pass1Rejected := make(map[string]bool, len(rej1))
	for _, r := range rej1 {
		if r.AssetID != "" {
			pass1Rejected[r.AssetID] = true
		}
	}
	valid, rej2 := runPass2(pass2Input{Accepted: survivors, Pass1Rejected: pass1Rejected, Existing: existing, Snap: snap})

	rejections := append(rej1, rej2...)
	slices.SortStableFunc(rejections, func(a, b rowRejection) int { return cmp.Compare(a.Line, b.Line) })

	return validation{
		cols:        cols,
		totalRows:   len(rows),
		blankRows:   blank,
		valid:       valid,
		rejections:  rejections,
		fingerprint: fingerprint(in.CSV, valid, rejections),
	}, nil
}

// fingerprint identifies one file together with what validating it produced:
// a SHA-256 over every cell, the ids that would be stored, and every
// rejection. It changes if the file changes or if the stored data changes in a
// way that alters the outcome (a parent appears or disappears, an id is
// taken). Fields are length-prefixed so no two inputs share an encoding.
func fingerprint(csv csvx.ParsedCSV, valid []AcceptedRow, rejections []rowRejection) string {
	h := sha256.New()
	put := func(parts ...string) {
		for _, p := range parts {
			fmt.Fprintf(h, "%d:%s|", len(p), p)
		}
		h.Write([]byte{'\n'})
	}
	put(csv.Header...)
	for _, r := range csv.Rows {
		put(strconv.Itoa(r.Line))
		put(r.Fields...)
	}
	ids := make([]string, 0, len(valid))
	for _, v := range valid {
		ids = append(ids, v.Asset.AssetID)
	}
	slices.Sort(ids)
	put("valid")
	put(ids...)
	put("rejected")
	for _, r := range rejections {
		put(strconv.Itoa(r.Line), r.AssetID, r.Reason)
	}
	return hex.EncodeToString(h.Sum(nil))
}

// Preview validates the file exactly as Import would and reports how many
// rows would be stored and which would be rejected, without writing anything:
// no assets, no audit rows.
func (s *ImportService) Preview(ctx context.Context, in ImportInput) (dto.ImportPreviewResponse, error) {
	v, err := s.validate(ctx, in)
	if err != nil {
		return dto.ImportPreviewResponse{}, err
	}
	logs.WithCtx(ctx).Info("import previewed",
		"file", in.Filename, "total_rows", v.totalRows, "importable_rows", len(v.valid), "rejected_rows", len(v.rejections))
	ignored := v.cols.ignored
	if ignored == nil {
		ignored = []string{}
	}
	return dto.ImportPreviewResponse{
		TotalRows:      v.totalRows,
		ImportableRows: len(v.valid),
		RejectedRows:   len(v.rejections),
		Rejections:     dtoRejections(v.rejections),
		IgnoredColumns: ignored,
		Fingerprint:    v.fingerprint,
	}, nil
}

// Import validates the file and commits the rows that pass; rejected rows
// (and anything beneath a rejected parent) are skipped and reported.
//
// Validation never writes: both passes compute the full accept/reject set
// first. Then, in one transaction, the accepted assets plus the audit rows are
// written, so a failure part-way leaves nothing behind. If ExpectedFingerprint
// is set and no longer matches, nothing is written and ErrPreviewStale is
// returned. Errors are *SchemaError, ErrNoDataRows, ErrPreviewStale,
// errx.ErrConflict (another writer collided with the commit), or an
// unexpected wrapped error; per-row problems are part of the response.
func (s *ImportService) Import(ctx context.Context, in ImportInput) (dto.ImportResponse, error) {
	started := time.Now()
	log := logs.WithCtx(ctx)

	v, err := s.validate(ctx, in)
	if err != nil {
		return dto.ImportResponse{}, err
	}
	if in.ExpectedFingerprint != "" && in.ExpectedFingerprint != v.fingerprint {
		log.Warn("import refused: result differs from the confirmed preview", "file", in.Filename)
		return dto.ImportResponse{}, ErrPreviewStale
	}

	committed := len(v.valid) > 0
	run := model.ImportRun{
		ID:           uuid.New(),
		Filename:     in.Filename,
		ClientIP:     in.ClientIP,
		TotalRows:    v.totalRows,
		ImportedRows: len(v.valid),
		RejectedRows: len(v.rejections),
		Committed:    committed,
	}

	err = repository.InTx(ctx, s.db, func(tx *gorm.DB) error {
		if committed {
			if err := repository.NewAssetRepository(tx).InsertBatch(ctx, orderParentsFirst(v.valid)); err != nil {
				return err
			}
		}
		runs := repository.NewImportRepository(tx)
		if err := runs.CreateRun(ctx, run); err != nil {
			return err
		}
		return runs.CreateRejections(ctx, run.ID, toModelRejections(run.ID, v.rejections))
	})
	if err != nil {
		return dto.ImportResponse{}, fmt.Errorf("commit import %s: %w", run.ID, err)
	}

	logRejections(ctx, run, v.rejections)
	log.Info("import finished",
		"import_id", run.ID, "file", in.Filename, "total_rows", run.TotalRows, "imported_rows", run.ImportedRows,
		"rejected_rows", run.RejectedRows, "committed", committed, "ignored_columns", len(v.cols.ignored),
		"blank_rows", v.blankRows, "dur_ms", time.Since(started).Milliseconds())

	return run.ToResponse(dtoRejections(v.rejections), v.cols.ignored), nil
}

// templateExamples are the fictional rows of the downloadable template: one
// small valid hierarchy showing how children point at parents and how dates,
// numbers and statuses are written.
var templateExamples = []map[string]string{
	{"asset_id": "EXAMPLE-SUB-001", "asset_type": "SUBSTATION", "asset_name": "Example Substation", "operational_status": "IN_SERVICE",
		"voltage_kv": "22", "manufacturer": "Example Manufacturer", "model": "SS-100", "serial_number": "EX-SS-0001", "commissioned_date": "2020-01-31"},
	{"asset_id": "EXAMPLE-TX-001-1", "parent_asset_id": "EXAMPLE-SUB-001", "asset_type": "TRANSFORMER", "asset_name": "Example Transformer 1",
		"operational_status": "IN_SERVICE", "voltage_kv": "22", "rating_kva": "1000", "manufacturer": "Example Manufacturer", "model": "TX-1000",
		"serial_number": "EX-TX-0001", "commissioned_date": "2020-06-15"},
	{"asset_id": "EXAMPLE-SWB-001-1", "parent_asset_id": "EXAMPLE-SUB-001", "asset_type": "SWITCHBOARD", "asset_name": "Example Switchboard A",
		"operational_status": "MAINTENANCE", "voltage_kv": "22", "commissioned_date": "2021-03-01"},
	{"asset_id": "EXAMPLE-PNL-001-1-1", "parent_asset_id": "EXAMPLE-SWB-001-1", "asset_type": "SWITCHBOARD_PANEL", "asset_name": "Example Panel A1",
		"operational_status": "OUT_OF_SERVICE"},
}

// Template returns the CSV a user can download, fill in and import: the header
// is generated from the importer's own column table (so it cannot drift from
// what the importer accepts) followed by the fictional example rows.
func (s *ImportService) Template() []byte {
	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	header := make([]string, len(assetFields))
	for i, f := range assetFields {
		header[i] = f.name
	}
	_ = w.Write(header)
	for _, ex := range templateExamples {
		row := make([]string, len(header))
		for i, name := range header {
			row[i] = ex[name]
		}
		_ = w.Write(row)
	}
	w.Flush()
	return buf.Bytes()
}

// ListImports returns one page of past imports, newest first.
func (s *ImportService) ListImports(ctx context.Context, req dto.ImportListRequest) (dto.ImportListResponse, error) {
	if err := req.Validate(); err != nil {
		return dto.ImportListResponse{}, err
	}
	runs, total, err := s.imports.List(ctx, req.Limit, req.Offset)
	if err != nil {
		return dto.ImportListResponse{}, err
	}
	out := dto.ImportListResponse{Total: int(total), Limit: req.Limit, Offset: req.Offset, Items: make([]dto.ImportSummary, 0, len(runs))}
	for _, r := range runs {
		out.Items = append(out.Items, r.ToSummary())
	}
	return out, nil
}

// GetImport re-hydrates a past import's result (rejections included);
// ignored_columns are not stored, so it is always empty here.
func (s *ImportService) GetImport(ctx context.Context, req dto.ImportIDRequest) (dto.ImportResponse, error) {
	if err := req.Validate(); err != nil {
		return dto.ImportResponse{}, err
	}
	uid, _ := uuid.Parse(req.ImportID) // Validate accepted it
	run, err := s.imports.GetRun(ctx, uid)
	if err != nil {
		return dto.ImportResponse{}, err
	}
	stored, err := s.imports.ListRejections(ctx, uid)
	if err != nil {
		return dto.ImportResponse{}, err
	}
	rejections := make([]dto.Rejection, 0, len(stored))
	for _, r := range stored {
		rejections = append(rejections, r.ToDTO())
	}
	return run.ToResponse(rejections, nil), nil
}

// referencedIDs returns every asset_id and parent_asset_id in the file,
// deduplicated — the ids whose presence in the database matters.
func referencedIDs(rows []rawRow) []string {
	seen := make(map[string]struct{}, len(rows)*2)
	for _, r := range rows {
		if id := r.assetID(); id != "" {
			seen[id] = struct{}{}
		}
		if p := r.parentID(); p != "" {
			seen[p] = struct{}{}
		}
	}
	ids := make([]string, 0, len(seen))
	for id := range seen {
		ids = append(ids, id)
	}
	return ids
}

func toModelRejections(runID uuid.UUID, in []rowRejection) []model.ImportRejection {
	out := make([]model.ImportRejection, 0, len(in))
	for _, r := range in {
		rej := model.ImportRejection{ImportRunID: runID, CSVRowNumber: r.Line, Reason: r.Reason}
		if r.AssetID != "" {
			id := r.AssetID
			rej.AssetID = &id
		}
		out = append(out, rej)
	}
	return out
}

func dtoRejections(in []rowRejection) []dto.Rejection {
	out := make([]dto.Rejection, 0, len(in))
	for _, r := range in {
		out = append(out, dto.Rejection{CSVRow: r.Line, AssetID: r.AssetID, Reason: r.Reason})
	}
	return out
}

// logRejections writes one Warn per rejected row (bounded) naming the CSV
// line, so an operator can find the same row the user is told about.
func logRejections(ctx context.Context, run model.ImportRun, rejections []rowRejection) {
	log := logs.WithCtx(ctx)
	for i, r := range rejections {
		if i == defined.MaxLoggedRejections {
			log.Warn("import rejections truncated in log",
				"import_id", run.ID, "logged", defined.MaxLoggedRejections, "total", len(rejections))
			return
		}
		log.Warn("import row rejected",
			"import_id", run.ID, "file", run.Filename, "line", r.Line, "asset_id", r.AssetID, "pass", r.Pass, "reason", r.Reason)
	}
}

// --------------------------------------------------------------------------
// CSV schema: header resolution and the declarative column table
// --------------------------------------------------------------------------

// dateLayouts are the accepted commissioned_date formats, tried in order:
// ISO (YYYY-MM-DD), then day-first D/M/YYYY and D/M/YY. Day and month may be
// written with or without a leading zero. The supplied data is day-first
// (many values have a day above 12, none a month above 12), so 3/4/10 is read
// as 3 April 2010. Two-digit years 00-68 mean 2000-2068, 69-99 mean 1969-1999
// (Go's convention).
var dateLayouts = []string{defined.DateFormat, "2/1/2006", "2/1/06"}

// fieldSpec declares one importable column. The importer contains no
// per-column logic beyond this table: header resolution, required
// checks, parsing and validation all iterate over assetFields, so a new
// column is one more entry here (plus its model field and DB column) —
// nothing else in the pipeline names a column.
type fieldSpec struct {
	name string
	// required means the column must exist in the header and every row
	// must have a non-blank value in it. Optional columns may be absent
	// from the file entirely, or blank on any row.
	required bool
	// check validates a non-blank, trimmed value. On success it returns
	// the function that stores the parsed value on the asset; otherwise a
	// description of the problem.
	check func(raw string, snap *LookupSnapshot) (set func(*model.Asset), problem string)
}

func textField(name string, required bool, set func(*model.Asset, string)) fieldSpec {
	return fieldSpec{name: name, required: required,
		check: func(raw string, _ *LookupSnapshot) (func(*model.Asset), string) {
			return func(a *model.Asset) { set(a, raw) }, ""
		}}
}

// numberField accepts finite decimals; min (when non-nil) is inclusive.
func numberField(name string, min *float64, set func(*model.Asset, float64)) fieldSpec {
	return fieldSpec{name: name,
		check: func(raw string, _ *LookupSnapshot) (func(*model.Asset), string) {
			v, err := strconv.ParseFloat(raw, 64)
			if err != nil || math.IsNaN(v) || math.IsInf(v, 0) {
				return nil, fmt.Sprintf("%s is not a valid number (%q)", name, raw)
			}
			if min != nil && v < *min {
				if *min == 0 {
					return nil, fmt.Sprintf("%s cannot be negative (%s)", name, raw)
				}
				return nil, fmt.Sprintf("%s must be at least %v (%s)", name, *min, raw)
			}
			return func(a *model.Asset) { set(a, v) }, ""
		}}
}

func dateField(name string, set func(*model.Asset, time.Time)) fieldSpec {
	return fieldSpec{name: name,
		check: func(raw string, _ *LookupSnapshot) (func(*model.Asset), string) {
			for _, layout := range dateLayouts {
				if t, err := time.Parse(layout, raw); err == nil {
					return func(a *model.Asset) { set(a, t) }, ""
				}
			}
			return nil, fmt.Sprintf("%s is not a valid date (%q; expected YYYY-MM-DD or D/M/YY)", name, raw)
		}}
}

// lookupField requires the value to be a member of a reference table
// (matched case-insensitively, stored as the canonical code).
func lookupField(name string, match func(*LookupSnapshot, string) (string, bool), allowed func(*LookupSnapshot) []string, set func(*model.Asset, string)) fieldSpec {
	return fieldSpec{name: name, required: true,
		check: func(raw string, snap *LookupSnapshot) (func(*model.Asset), string) {
			code, ok := match(snap, raw)
			if !ok {
				return nil, fmt.Sprintf("invalid %s %q (allowed: %s)", name, raw, strings.Join(allowed(snap), ", "))
			}
			return func(a *model.Asset) { set(a, code) }, ""
		}}
}

var zero = 0.0

// assetFields is the import schema. Order is irrelevant to the CSV — columns
// are found by header name — and only fixes the order problems are reported in.
var assetFields = []fieldSpec{
	textField("asset_id", true, func(a *model.Asset, v string) { a.AssetID = v }),
	lookupField("asset_type", (*LookupSnapshot).MatchAssetType, (*LookupSnapshot).AssetTypes,
		func(a *model.Asset, v string) { a.AssetType = v }),
	textField("asset_name", true, func(a *model.Asset, v string) { a.AssetName = v }),
	lookupField("operational_status", (*LookupSnapshot).MatchStatus, (*LookupSnapshot).Statuses,
		func(a *model.Asset, v string) { a.OperationalStatus = v }),
	textField("parent_asset_id", false, func(a *model.Asset, v string) { a.ParentAssetID = &v }),
	numberField("voltage_kv", nil, func(a *model.Asset, v float64) { a.VoltageKV = &v }),
	numberField("rating_kva", &zero, func(a *model.Asset, v float64) { a.RatingKVA = &v }),
	textField("manufacturer", false, func(a *model.Asset, v string) { a.Manufacturer = &v }),
	textField("model", false, func(a *model.Asset, v string) { a.Model = &v }),
	textField("serial_number", false, func(a *model.Asset, v string) { a.SerialNumber = &v }),
	dateField("commissioned_date", func(a *model.Asset, v time.Time) { a.CommissionedDate = &v }),
}

// fieldIndex maps a column name to its position in assetFields.
var fieldIndex = func() map[string]int {
	m := make(map[string]int, len(assetFields))
	for i, f := range assetFields {
		m[f.name] = i
	}
	return m
}()

func mustFieldIndex(name string) int {
	i, ok := fieldIndex[name]
	if !ok {
		panic("service: no import field named " + name)
	}
	return i
}

var (
	idxAssetID  = mustFieldIndex("asset_id")
	idxParentID = mustFieldIndex("parent_asset_id")
)

// RequiredColumnCount is how many columns a file must contain at minimum;
// the handler passes it to csvx as a cheap wrong-delimiter check without
// hardcoding a column count.
func RequiredColumnCount() int {
	n := 0
	for _, f := range assetFields {
		if f.required {
			n++
		}
	}
	return n
}

// SchemaError means the CSV parsed but its header cannot be mapped onto
// the asset schema: a required column is absent, or a known column
// appears twice. It is a whole-file problem, not a per-row rejection.
type SchemaError struct {
	Missing   []string // required columns not found
	Duplicate []string // known columns that appear more than once
	Found     []string // the header as uploaded, for the message
}

func (e *SchemaError) Error() string {
	var parts []string
	if len(e.Missing) > 0 {
		parts = append(parts, "missing required column(s): "+strings.Join(e.Missing, ", "))
	}
	if len(e.Duplicate) > 0 {
		parts = append(parts, "duplicate column(s): "+strings.Join(e.Duplicate, ", "))
	}
	return fmt.Sprintf("CSV header does not match the asset schema — %s (found columns: %s)",
		strings.Join(parts, "; "), strings.Join(e.Found, ", "))
}

// columnMap says where each schema field lives in the uploaded header.
type columnMap struct {
	pos     []int    // pos[i] is the column of assetFields[i], or -1 if absent
	ignored []string // header columns that are not part of the schema
}

// resolveColumns matches the header to the schema by name. Column order,
// extra columns and absent optional columns are all fine; a missing
// required column or a repeated known column is a *SchemaError.
func resolveColumns(header []string) (columnMap, error) {
	cm := columnMap{pos: make([]int, len(assetFields))}
	for i := range cm.pos {
		cm.pos[i] = -1
	}
	var duplicates []string
	seenIgnored := map[string]bool{}
	for col, h := range header {
		name := csvx.NormalizeHeader(h)
		if i, known := fieldIndex[name]; known {
			if cm.pos[i] != -1 {
				duplicates = append(duplicates, name)
				continue
			}
			cm.pos[i] = col
			continue
		}
		if trimmed := strings.TrimSpace(h); trimmed != "" && !seenIgnored[name] {
			seenIgnored[name] = true
			cm.ignored = append(cm.ignored, trimmed)
		}
	}

	var missing []string
	for i, f := range assetFields {
		if f.required && cm.pos[i] == -1 {
			missing = append(missing, f.name)
		}
	}
	if len(missing) > 0 || len(duplicates) > 0 {
		return columnMap{}, &SchemaError{Missing: missing, Duplicate: duplicates, Found: trimAll(header)}
	}
	return cm, nil
}

func trimAll(in []string) []string {
	out := make([]string, len(in))
	for i, s := range in {
		out[i] = strings.TrimSpace(s)
	}
	return out
}

// rawRow is one CSV row projected onto the schema: vals[i] is the trimmed
// value of assetFields[i] ("" when blank or the column is absent).
type rawRow struct {
	Line int
	vals []string
}

func (r rawRow) assetID() string  { return r.vals[idxAssetID] }
func (r rawRow) parentID() string { return r.vals[idxParentID] }

// mapRows projects parsed rows onto the schema. Rows that are blank in
// every column (Excel loves to export trailing ",,,,") are skipped — they
// carry no data to import or reject — and counted in blank.
func mapRows(rows []csvx.Row, cm columnMap) (out []rawRow, blank int) {
	out = make([]rawRow, 0, len(rows))
	for _, row := range rows {
		if row.IsBlank() {
			blank++
			continue
		}
		vals := make([]string, len(assetFields))
		for i, col := range cm.pos {
			if col >= 0 && col < len(row.Fields) {
				vals[i] = strings.TrimSpace(row.Fields[col])
			}
		}
		out = append(out, rawRow{Line: row.Line, vals: vals})
	}
	return out, blank
}

// --------------------------------------------------------------------------
// Pass 1: field-level validation, one row at a time
// --------------------------------------------------------------------------

// AcceptedRow is a CSV row that survived validation, still tagged with
// its original line so a later pass can report against it.
type AcceptedRow struct {
	Line  int
	Asset model.Asset // ParentAssetID is carried through unchecked — pass 2's job
}

// rowRejection is one rejected row, from either pass.
type rowRejection struct {
	Line    int
	AssetID string
	Reason  string
	Pass    int
}

type pass1Result struct {
	asset    model.Asset
	problems []string
}

// runPass1 is the field-level pass (docs/ARCHITECTURE.md section 4):
// every row is checked on its own, independent of every other row except
// for asset_id uniqueness, which is settled by one sequential scan before
// the per-row work fans out across goroutines. Output order follows input
// order regardless of scheduling.
func runPass1(ctx context.Context, rows []rawRow, snap *LookupSnapshot) ([]AcceptedRow, []rowRejection, error) {
	dups := duplicateIDs(rows)

	results := make([]pass1Result, len(rows))
	err := parallel.Chunks(ctx, len(rows), func(lo, hi int) {
		for i := lo; i < hi; i++ {
			results[i] = validateRow(rows[i], snap, dups)
		}
	})
	if err != nil {
		return nil, nil, err
	}

	accepted := make([]AcceptedRow, 0, len(rows))
	var rejected []rowRejection
	for i, r := range results {
		if len(r.problems) == 0 {
			accepted = append(accepted, AcceptedRow{Line: rows[i].Line, Asset: r.asset})
			continue
		}
		rejected = append(rejected, rowRejection{
			Line:    rows[i].Line,
			AssetID: rows[i].assetID(),
			Reason:  strings.Join(r.problems, "; "),
			Pass:    1,
		})
	}
	return accepted, rejected, nil
}

// duplicateIDs maps each asset_id that appears on more than one row to
// every line it appears on. Every occurrence is rejected: nothing says
// which one is authoritative.
func duplicateIDs(rows []rawRow) map[string][]int {
	seen := make(map[string][]int, len(rows))
	for _, r := range rows {
		if id := r.assetID(); id != "" {
			seen[id] = append(seen[id], r.Line)
		}
	}
	for id, lines := range seen {
		if len(lines) < 2 {
			delete(seen, id)
		}
	}
	return seen
}

// validateRow checks one row against the schema and reports every
// problem it has, so the user can fix a row in one pass.
func validateRow(row rawRow, snap *LookupSnapshot, dups map[string][]int) pass1Result {
	var (
		problems []string
		setters  []func(*model.Asset)
	)
	for i, f := range assetFields {
		raw := row.vals[i]
		if raw == "" {
			if f.required {
				problems = append(problems, "missing required field: "+f.name)
			}
			continue
		}
		set, problem := f.check(raw, snap)
		if problem != "" {
			problems = append(problems, problem)
			continue
		}
		setters = append(setters, set)
	}
	if id := row.assetID(); id != "" {
		if lines, isDup := dups[id]; isDup {
			problems = append(problems, fmt.Sprintf("duplicate asset_id %q (also on rows %s)", id, otherLines(lines, row.Line)))
		}
	}
	if len(problems) > 0 {
		return pass1Result{problems: problems}
	}
	var a model.Asset
	for _, set := range setters {
		set(&a)
	}
	return pass1Result{asset: a}
}

func otherLines(lines []int, self int) string {
	var parts []string
	for _, l := range lines {
		if l != self {
			parts = append(parts, strconv.Itoa(l))
		}
	}
	return strings.Join(parts, ", ")
}

// --------------------------------------------------------------------------
// Pass 2: hierarchy validation, and ordering for insert
// --------------------------------------------------------------------------

// pass2Input is everything hierarchy validation needs. It is a pure
// function of these values — no database access — so it is unit-testable
// with plain maps.
type pass2Input struct {
	Accepted []AcceptedRow // pass-1 survivors, in file order
	// Pass1Rejected holds the non-empty asset_ids of file rows pass 1
	// rejected: a child pointing at one is rejected too (cascade).
	Pass1Rejected map[string]bool
	Existing      map[string]string // asset_id -> asset_type already stored in the database
	Snap          *LookupSnapshot
}

// runPass2 is the hierarchy pass (docs/ARCHITECTURE.md section 4). It
// works over the candidate set "assets already in the database ∪ file
// rows that survived pass 1", so row order in the file is irrelevant: a
// child may precede its parent.
//
// It is deliberately sequential. The per-row checks are a handful of map
// lookups, and cycle detection and cascading share state across rows;
// fanning that out would add locking or races for no measurable gain.
func runPass2(in pass2Input) (accepted []AcceptedRow, rejected []rowRejection) {
	n := len(in.Accepted)
	byID := make(map[string]int, n) // survivors only; pass 1 guarantees unique ids
	for i, row := range in.Accepted {
		byID[row.Asset.AssetID] = i
	}

	own := make([]string, n) // problems found by the per-row checks
	parentOf := func(i int) string {
		if p := in.Accepted[i].Asset.ParentAssetID; p != nil {
			return *p
		}
		return ""
	}

	// Checks 0-4, one row at a time.
	for i, row := range in.Accepted {
		a := row.Asset
		parent := parentOf(i)

		if _, stored := in.Existing[a.AssetID]; stored {
			own[i] = fmt.Sprintf("asset_id %q already exists in the database", a.AssetID)
			continue
		}
		isRoot := in.Snap.IsRootType(a.AssetType)
		switch {
		case isRoot && parent != "":
			own[i] = fmt.Sprintf("%s is a root type and must not have a parent_asset_id (got %q)", a.AssetType, parent)
		case !isRoot && parent == "":
			own[i] = fmt.Sprintf("%s requires a parent_asset_id", a.AssetType)
		case parent == a.AssetID:
			own[i] = "asset cannot be its own parent"
		case parent != "":
			parentType, found := in.Existing[parent]
			if !found {
				if j, inFile := byID[parent]; inFile {
					parentType, found = in.Accepted[j].Asset.AssetType, true
				}
			}
			switch {
			case found:
				allowed := in.Snap.AllowedParents(a.AssetType)
				if !slices.Contains(allowed, parentType) {
					own[i] = fmt.Sprintf("parent %q is a %s, but a %s must have a parent of type %s",
						parent, parentType, a.AssetType, strings.Join(allowed, " or "))
				}
			case in.Pass1Rejected[parent]:
				// Handled by the cascade below.
			default:
				own[i] = fmt.Sprintf("parent %q does not exist in the database or file", parent)
			}
		}
	}

	// Check 5: cycle detection. Each row has at most one parent, so the
	// graph is functional and one colouring sweep finds every cycle in O(n).
	//
	// It runs over every in-file parent link, not only rows that passed the
	// per-row checks: type rules are acyclic (a switchboard may only sit
	// under a substation), so a cycle could never pass the parent-type check
	// and would always be mislabelled as "wrong parent type". A cycle
	// member's reason therefore leads with the cycle and appends any other
	// problem the row has.
	next := make([]int, n)
	for i := range next {
		next[i] = -1
		p := parentOf(i)
		if _, stored := in.Existing[p]; p == "" || stored || p == in.Accepted[i].Asset.AssetID {
			continue // chain ends at a root or an already-stored (acyclic) asset; self-parent has its own reason
		}
		if j, ok := byID[p]; ok {
			next[i] = j
		}
	}
	reason := slices.Clone(own)
	const (
		unvisited = iota
		walking
		done
	)
	colour := make([]uint8, n)
	for start := 0; start < n; start++ {
		if colour[start] != unvisited {
			continue
		}
		var path []int
		cur := start
		for cur != -1 && colour[cur] == unvisited {
			colour[cur] = walking
			path = append(path, cur)
			cur = next[cur]
		}
		if cur != -1 && colour[cur] == walking {
			cycle := path[slices.Index(path, cur):]
			for k, member := range cycle {
				ids := make([]string, 0, len(cycle)+1)
				for off := 0; off <= len(cycle); off++ {
					ids = append(ids, in.Accepted[cycle[(k+off)%len(cycle)]].Asset.AssetID)
				}
				reason[member] = "cycle detected: " + strings.Join(ids, " -> ")
				if own[member] != "" {
					reason[member] += "; " + own[member]
				}
			}
		}
		for _, p := range path {
			colour[p] = done
		}
	}

	// Check 6: cascade. A row whose own checks passed is still rejected
	// when an ancestor was rejected; the reason names the nearest ancestor
	// that was rejected in its own right (the root cause), not merely the parent.
	cause := make([]string, n)
	resolved := make([]bool, n)
	var nearestRejected func(i int) string
	nearestRejected = func(i int) string {
		if reason[i] != "" {
			return in.Accepted[i].Asset.AssetID
		}
		if resolved[i] {
			return cause[i]
		}
		c := ""
		if p := parentOf(i); p != "" {
			if _, stored := in.Existing[p]; !stored {
				if j, ok := byID[p]; ok {
					c = nearestRejected(j)
				} else if in.Pass1Rejected[p] {
					c = p
				}
			}
		}
		cause[i], resolved[i] = c, true
		return c
	}
	// Cascade reasons go in a separate slice: nearestRejected must keep
	// reading only the rows rejected in their own right, or the ancestor it
	// names would depend on the order rows happen to appear in the file.
	final := slices.Clone(reason)
	for i := range in.Accepted {
		if reason[i] == "" {
			if c := nearestRejected(i); c != "" {
				final[i] = fmt.Sprintf("ancestor %s was rejected", c)
			}
		}
	}

	for i, row := range in.Accepted {
		if final[i] == "" {
			accepted = append(accepted, row)
			continue
		}
		rejected = append(rejected, rowRejection{Line: row.Line, AssetID: row.Asset.AssetID, Reason: final[i], Pass: 2})
	}
	return accepted, rejected
}

// orderParentsFirst returns the rows' assets ordered so every asset comes
// after its parent (when the parent is in the same file). assets.
// parent_asset_id is a plain foreign key, so a child inserted before its
// parent would be rejected by Postgres. Order within one depth follows
// the file. Callers pass rows that already survived hierarchy
// validation, so parent chains are acyclic and every file parent is present.
func orderParentsFirst(rows []AcceptedRow) []model.Asset {
	byID := make(map[string]int, len(rows))
	for i, r := range rows {
		byID[r.Asset.AssetID] = i
	}
	depth := make([]int, len(rows))
	known := make([]bool, len(rows))
	var depthOf func(i int) int
	depthOf = func(i int) int {
		if known[i] {
			return depth[i]
		}
		d := 0
		if p := rows[i].Asset.ParentAssetID; p != nil {
			if j, inFile := byID[*p]; inFile {
				d = depthOf(j) + 1
			}
		}
		depth[i], known[i] = d, true
		return d
	}

	order := make([]int, len(rows))
	for i := range order {
		order[i] = i
		depthOf(i)
	}
	slices.SortStableFunc(order, func(a, b int) int { return depth[a] - depth[b] })

	out := make([]model.Asset, len(rows))
	for k, i := range order {
		out[k] = rows[i].Asset
	}
	return out
}

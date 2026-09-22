# Import edge-case fixtures — test results

Manual QA fixtures for the CSV import validation algorithm (`docs/ARCHITECTURE.md`
section 4). Each file isolates one or a few validation rules so a rejection reason
can be attributed to a specific rule at a glance. Run 2026-09-22 against the local
`docker compose` stack (backend + Postgres 16, healthy) by posting each file to
`POST /api/imports/preview` (validates, writes nothing) with `x-api-key` set.
Every result below matched the expected outcome — no discrepancies found.

## Preview results (validate-only, no writes)

| File | Exercises | Result |
|---|---|---|
| `01_missing_required_fields.csv` | blank `asset_id`/`asset_type`/`asset_name`/`operational_status` | 6 rows, 2 importable, 4 rejected — each names the missing field (`missing required field: asset_id`, etc.) |
| `02_duplicate_asset_ids.csv` | same `asset_id` on 3 rows | 5 rows, 2 importable, all 3 duplicates rejected, each naming the other two rows (`duplicate asset_id "DUP-001" (also on rows 4, 5)`) |
| `03_bad_numbers_and_dates.csv` | non-numeric `voltage_kv`/`rating_kva`, negative `rating_kva`, impossible date (31/2), garbage date text; plus passing cases: 2-digit year pivot (99→1999, 05→2005), `YYYY-MM-DD`, day>12, `rating_kva=0` | 11 rows, 6 importable, 5 rejected exactly as expected; all passing-case rows importable |
| `04_invalid_lookup_values.csv` | unknown `asset_type`/`operational_status`; lower/mixed-case values that should normalize | 4 rows, 2 importable, 2 rejected (`invalid asset_type "GENERATOR" (allowed: LV_BOARD, SUBSTATION, SWITCHBOARD, SWITCHBOARD_PANEL, TRANSFORMER)`, similarly for status); lowercase/mixed-case rows passed and normalized |
| `05_hierarchy_and_cycles.csv` | root-with-parent, non-root-without-parent, self-parent, dangling parent, 2× wrong-parent-type, 2-node cycle, 3-node cycle, 2-level cascading rejection | 16 rows, 2 importable, 14 rejected; each hierarchy rule fired with the exact documented message; cycle rows lead with `cycle detected: A -> B -> A` (plus the type-pairing reason, since the cycle members are also mistyped by construction); the cascaded child *and* grandchild both report `ancestor SS-CAS-BADSTATUS was rejected` — pointing at the root cause, not the immediate parent |
| `06_structural_missing_column.csv` | required column (`operational_status`) absent from header | **HTTP 422**, `missing required column(s): operational_status`, names all found columns |
| `07_structural_duplicate_column.csv` | `asset_type` header repeated | **HTTP 422**, `duplicate column(s): asset_type` |
| `08_structural_ragged_row.csv` | too-few-fields row, then an unquoted-comma row that shifts columns | **HTTP 422**, `record on line 3: wrong number of fields` — reader aborts at the *first* ragged line, so the second bad row is never reached (expected: Go's `csv.Reader` stops on the first structural error) |
| `09_blank_rows_and_unknown_columns.csv` | fully blank row; unknown `region` column | `total_rows: 2` (blank row skipped, not counted), `ignored_columns: ["region"]`, both real rows importable |
| `10_messy_formatting.csv` | UTF-8 BOM, CRLF line endings, quoted multi-line cell, padded whitespace inside quoted values | 3 rows, all 3 importable, 0 rejected — real-world formatting handled transparently |
| `11_duplicate_of_existing_db_asset.csv` | `asset_id` (`SS-001`) that already exists in the DB (Rule 0) | 2 rows, 1 importable, 1 rejected: `asset_id "SS-001" already exists in the database` — **depends on DB state**: only rejects if `backend/testdata/grid_assets.csv` (or another file containing `SS-001`) was already committed; confirmed true against this dev DB |

## Commit-path verification (`POST /api/imports`)

Not covered by the files above since `/preview` never writes. Verified separately
with small ad-hoc CSVs (not kept in this directory — their asset IDs now exist in
the dev DB, so re-running them would just hit Rule 0):

1. **Normal commit** — preview a 2-row file (`SS-COMMIT-1` substation +
   `TX-COMMIT-1` child transformer), then confirm with the preview's
   `expected_fingerprint`. Result: `committed: true`, `imported_rows: 2`; both
   assets confirmed present via `/api/assets/search` with the parent link intact.
2. **412 on stale fingerprint** — confirm the same shape of file with a
   deliberately wrong `expected_fingerprint`. Result: **HTTP 412**,
   `the file's validation result changed since the preview; review it again`,
   nothing written (confirmed via search). Re-previewing for the real fingerprint
   and confirming again then succeeded normally.
3. **409 on a race** — fired two concurrent `POST /api/imports` for the same new
   `asset_id` with no `expected_fingerprint`. Result: one request **200**
   (committed), the other **409** (`the stored data changed while the request was
   being processed; please retry`); confirmed exactly one copy of the asset exists
   afterward — no half-write, no duplicate.

All three matched `docs/ARCHITECTURE.md` section 4 exactly.

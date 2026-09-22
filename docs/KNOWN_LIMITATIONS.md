# Known Limitations

## Hierarchy enforcement is application-layer, not a DB constraint

The permitted-parent-type *pairs* (e.g. a `TRANSFORMER`'s parent must be
a `SUBSTATION`) now live in a table (`asset_type_parent_rules`) for
scalability — same reasoning as `asset_types`/`operational_statuses` —
but *enforcement* (matching, cycle detection, cascading rejection) still
happens in the Go backend at import time, not via a Postgres
trigger/constraint, and validation loads the pairs into an in-memory
cache rather than querying per row (see the next section). This is a
deliberate trade-off (see `docs/ARCHITECTURE.md` section 2/4
[ASSUMPTION]): only ever exercised through the write paths this app
has (CSV import, plus deleting one asset that has no children; there is no edit). A raw `INSERT`
issued directly against the database would bypass these checks
regardless of where the pair data lives; nothing in this system performs
raw inserts outside the import path.

## Lookup tables are read through a snapshot, not live

`asset_types`/`operational_statuses`/`asset_type_parent_rules` are lookup
tables so a new type/status/pair is a plain `INSERT`, and validation reads
them through an immutable in-memory snapshot (`service.LookupCache`) rather
than querying per row or using Go constants. The snapshot is loaded at
startup and **refreshed at the start of every import**, so an import always
validates against the current tables. What is *not* live: `GET /api/lookups`
and the `type` check on `GET /api/assets/search` read the snapshot without
refreshing, so a row `INSERT`ed straight into a lookup table shows up there
after the next import or a restart. There is no background refresher; none
was needed because the only consumer that must be current (import) refreshes
itself.

## `import_runs.client_ip` is audit-only, not a security control

nginx (`frontend/nginx.conf`) sets `X-Forwarded-For`/`X-Real-IP` on
`/api/*`, and the import handler (`httpx.ClientIP`) trusts nginx as
the sole in-path proxy and takes the last `X-Forwarded-For` hop. But
`docker-compose.yml` also publishes the backend's port directly to the
host (`ports: - "${BACKEND_PORT}:${BACKEND_PORT}"`), so a caller can hit
the backend directly, bypassing nginx entirely, and set an arbitrary
`X-Forwarded-For` header with nothing in that path to strip or verify
it. `client_ip` is therefore only reliable as a best-effort "who ran
this" breadcrumb for local debugging/traceability — consistent with
this app having no auth at all — and must never be treated as a
trustworthy identity or used for any access-control decision.

## Import: skipped rows are not stored

Rows that fail validation, and rows under a rejected parent, are skipped (the user sees
the list before confirming). Nothing is saved for them: fix them in the file and import
only the corrected rows, because rows that already exist are rejected as duplicates and
never updated. There is no "edit" and no "update if exists". A rejected row's original
cell values are not stored, only its line number, asset id and reason.

## Import behaviours worth knowing

- **Re-importing a file that already committed rejects every row** (`asset_id
  ... already exists in the database`). There is no upsert: updating stored
  assets is out of scope ("no CRUD beyond import"), and upserting parents
  could create cycles through rows already in the database. To change data,
  delete the assets (leaves first; a parent with children is refused) and import
  the corrected rows again.
- **Concurrent imports use optimistic concurrency, not a lock.** Validation
  reads the database, then the commit inserts; if another import commits
  conflicting assets in between, the loser's insert violates the primary/
  foreign key and the caller gets `409` (nothing half-written, transaction
  rolled back). Retrying revalidates against the new state.
- **Unknown CSV columns are ignored, not stored.** They are reported in
  `ignored_columns` so nothing is dropped silently, but there is no
  free-form attributes column. Persisting them would mean a `JSONB` column
  and a migration — a reasonable next step if extra attributes matter.
- **Ragged rows fail the whole file (`422`, with the line)** rather than being
  padded/truncated. That is deliberate: an unquoted comma in a value would
  otherwise shift every later column and store wrong data. Trailing all-blank
  rows are skipped.
- **The rejection list is not paginated or capped** in the response (only the
  per-row *log lines* are capped, at 50 per import). A 10 MiB file of nothing
  but bad rows returns a large JSON body; the client is expected to page it.
- **Numbers are parsed as `float64`** before being stored in `NUMERIC`
  columns, so values with more than ~15 significant digits lose precision.
  Grid ratings and voltages are nowhere near that.
- **`GET /api/assets/roots` and `/children` are not paginated.** Fine at this
  scale (hundreds of assets, a handful of children per parent); a real
  deployment would page them.
- **The rate limiter is per process and per (IP, path)** (100 requests / 10 s
  by default), so a burst of uploads from one address can hit `429`.
- **Swagger is open.** Anyone who can reach the backend port can read the docs, but not call
  the API without the key.

- **Only the documented verbs are answered.** `HEAD` is not served implicitly
  for `GET` routes (the original stdlib mux did), and an unknown path or verb
  is a JSON `404`/`405`. Nothing in the app relies on `HEAD`.
- **Generated frontend types trust the spec.** They are only as accurate as
  the swag annotations; `router/swagger_test.go` catches a route missing from
  the spec but not a wrong field type in an annotation.

## Not done, and what I would do next

- **No CI/CD, by design.** Only a local build and run is needed (`ASSUMPTIONS.md`), so no pipeline exists
  and nothing runs the tests automatically: run the backend and frontend tests by hand before a release
  (`docs/DEPLOYMENT.md`). There is no registry, remote target or automated deploy; a second machine means
  cloning the repository and running Compose there.
- **End-to-end and screen-reader tests.** Behaviour is covered by unit, integration and
  component tests, and the pages were looked at in headless Chrome, but nothing drives a real
  import through a browser.
- **Users and an audit trail for deletes**, so the delete is guarded by more than a shared key
  and shows on the activity page.
- **Editing**, an upsert on import, and persisting extra CSV columns (`JSONB`).

## Frontend

- **The tree and overview cards show only the total beneath a top-level asset** (for example 15), not a
  per-type breakdown; the breakdown is on the asset's details page ("Contents by type, at every level").
- **Only lightly checked in a browser.** Behaviour is covered by Vitest and a run against the
  real API, and the main pages were looked at in headless Chrome (light and dark, 1440 px and
  390 px). Focus rings and drag-and-drop have not been looked at on screen (the delete dialog,
  the import review and report screens, the loading and error states and the mobile drawer were
  captured in headless Chrome on 2026-09-21 for the manual), and there is no end-to-end or
  screen-reader test.
- **The final import report is shown once**, from memory; a past import can be reopened
  from Import activity (which reads `GET /api/imports/{id}`).
- **Search shows at most 20 matches** (`SEARCH_LIMIT`); when the API reports `truncated`
  the list says so.
- **A pending import review is lost on refresh**: the chosen file is held in memory only, so it
  must be chosen again.
- **The tree does not virtualize.** Fine for a few hundred assets; a very wide node
  (thousands of children) would render them all.

## Deleting and the API being open

- **The whole `/api` is guarded by one shared API key, not by user accounts.** It requires
  `x-api-key` (constant-time compare, header only, never a query parameter; 401 if wrong, 403
  when the server has no key configured, so it fails closed and the app does not work at all
  without `API_KEY`). The frontend's nginx adds the key to every proxied request, so it is not in
  the browser or the JS bundle, **but that also means anyone who can use the UI can use every
  feature, including delete**: the key protects the backend API from direct callers (for example the
  published port 8080), not the UI from its users. Real protection of the UI would need logins, which
  are out of scope. The key sits in the nginx config inside the container and travels in a header, so
  outside a local setup it needs TLS; the rate limiter is the only brake on guessing it. Only
  `/healthz` and the Swagger docs are open.
- **Deletions are logged, not recorded in the database**, so the Import activity page cannot
  show them.
- **A race between preview and confirm** is caught by the fingerprint (412) or, in the
  narrow window after the second validation, by the database keys (409). Both refuse the
  whole commit: nothing is half-stored.
- **Ambiguous dates are read day-first.** `3/4/10` is 3 April 2010. If a supplier sends
  month-first files with all days ≤ 12 they will be misread without any error.

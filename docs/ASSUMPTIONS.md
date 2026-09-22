# Assumptions

Every place a judgment call was made instead of the brief being followed
literally, in one list. Most of these are also marked **[ASSUMPTION]**
inline in [`ARCHITECTURE.md`](ARCHITECTURE.md) next to the
relevant section; see that file for the full reasoning. `KNOWN_LIMITATIONS.md`
covers the trade-offs that follow from these, plus anything left undone.

## Scope & timeline

- **7 working days, not 5.** The brief's day count and its scope don't
  reconcile at 5 days; this plan builds against a 7-day reading.
- **No auth, no RBAC, no CRUD beyond CSV import** — read literally from
  section 8 of the brief, not softened "for realism." The owner-requested exceptions (read-only
  lists, deleting one asset, and one shared API key on every `/api` endpoint) are recorded at the
  end of this file.
- **Explicitly out of scope (brief, section 8):** authentication or user accounts; editing and
  deleting individual assets; GIS maps; telemetry or time-series data; power-flow calculations;
  cloud deployment or Kubernetes; role-based access control; pixel-perfect visual design. Three of
  these were later relaxed at the owner's request, and each is recorded at the end of this file:
  deleting one childless asset, the shared API key (still no accounts, roles or login screen), and
  the read-only list/stats endpoints. Nothing else on the list has been built.
- **[ASSUMPTION] Only a local build and run is needed, so there is no CI/CD pipeline.** Because cloud
  deployment and Kubernetes are out of scope, "deploy" means `docker compose up --build` on the owner's own
  machine. A CI pipeline (tests on every push, build and SSH deploy on a tag) was first written and never
  run, since no runner existed; at the owner's request (2026-09-21) it was removed entirely, together with its
  variables script. Tests are run by hand (`go test -race ./...`, `npm test`, `npm run build`), and the build,
  run, update and troubleshooting commands are in `docs/DEPLOYMENT.md`. Trade-off: nothing runs the tests
  automatically, so they must be run before a release (`KNOWN_LIMITATIONS.md`).

## Data model & validation (docs/ARCHITECTURE.md sections 2 & 4)

- **`asset_type`/`operational_status` are lookup tables**
  (`asset_types`/`operational_statuses`, natural-key `code TEXT PRIMARY
  KEY`), not native Postgres `ENUM` types — adding a new type/status
  later is a plain seed-row `INSERT`, not an `ALTER TYPE`/schema
  migration. The `assets` FK column names and the Go
  `AssetType`/`OperationalStatus` const types are unchanged. See
  `docs/ARCHITECTURE.md` section 2 and `docs/KNOWN_LIMITATIONS.md` for
  the validator-cache trade-off this introduces.
- **Permitted parent-type pairs live in a table too**
  (`asset_type_parent_rules`), for the same scalability reason —
  **enforcement** (parent existence, type-pairing, cycle detection,
  cascading rejection) stays in the application layer, not via a
  Postgres trigger/constraint, and only exercised through the CSV
  import path.
- **Every table has `created_at` and `updated_at`**, the latter kept
  current by one shared `set_updated_at()` trigger function reused
  per-table — including `import_rejections`, an append-only audit table
  with no update code path today; the trigger exists for schema-wide
  uniformity.
- **`import_runs.client_ip` (`INET`, `NOT NULL`) stands in for a user
  id**, since there's no auth/user system to attribute an import to.
  It's informational/audit-only, not a security control — see
  `docs/KNOWN_LIMITATIONS.md`.
- **Import is check → review → confirm, then the valid rows are committed** (this
  replaced an earlier all-or-nothing policy, decided before the supplied file was
  available). The supplied `grid_assets.csv` has 222 rows and about 17 seeded bad ones
  (orphans, a cycle, wrong parent types, a bad type, status and date, a negative rating,
  a missing id and a missing name), so all-or-nothing would never have loaded it.
  Preview writes nothing; confirm validates again and stores the rows that pass in one
  transaction; a fingerprint refuses a confirm whose outcome changed (412), and the
  database's key checks turn any remaining race into 409. The user's explicit choice to
  import is consent to skip the rejected rows. See `docs/ARCHITECTURE.md` section 4.
- **Cascading rejection**: a row whose parent was rejected (even
  transitively) is itself rejected with `"ancestor <id> was rejected"`,
  rather than becoming an orphan or a fake root.
- **`csv_row` in rejection reports is the original 1-indexed file row**
  (including header), not a post-sort position.

- **Validation reads the database but never writes to it.** The two passes
  are pure functions of the parsed rows plus two reads done up front — the
  lookup-table snapshot and one bulk "which of these ids already exist"
  query — so the rule that validation never queries the database per row is honoured
  in the sense that matters: nothing is written until the full accept/reject
  set is known, and nothing is queried per row.
- **CSV columns are resolved by header name from one declarative schema**
  (`service/import.go`), so order, extra columns and absent optional
  columns are all tolerated and no column position or count is hardcoded. See
  `docs/ARCHITECTURE.md` section 4.1.
- **Permitted types/statuses/parent pairs come from the lookup tables** via an
  atomically-swapped snapshot; the Go `AssetType`/`OperationalStatus`
  constants were removed. "Root type" is derived (a type with no parent rule).
- **Duplicate `asset_id`s within a file reject every occurrence**, and an
  `asset_id` already stored in the database is rejected (no upsert).
- **Cycle detection runs over all in-file parent links** (not only rows that
  passed the per-row checks), because type rules are acyclic and a cycle could
  otherwise never be reported as one. Cascade reasons name the nearest
  ancestor rejected in its own right.
- **`commissioned_date` accepts `YYYY-MM-DD`, `D/M/YYYY` and `D/M/YY`, always day-first.**
  The supplied file writes dates as `15/2/09`: 122 values have a day above 12, none has a
  month above 12, so day-first is the only consistent reading. A month-first value such
  as `12/31/2020` is rejected, and impossible dates (`31/2/10`, `2026-13-41`) stay
  rejected. Two-digit years use Go's pivot (00–68 → 2000–2068, 69–99 → 1969–1999).
  Stored and returned as `YYYY-MM-DD`.

## API contract

- **All JSON keys are `snake_case`**, not Go's default camelCase/PascalCase
  — every handler sets explicit `json:"snake_case_name"` tags.
- **Swagger/OpenAPI docs are open, with no API key.** They were first gated by the shared
  key (an S1 request), then opened at the owner's request: the endpoints they describe are
  already open at the time, so gating the description protected nothing. The endpoints
  themselves were then put behind the key (below); the docs stay open so a reader can see the API,
  and the page's **Authorize** button lets them try it with the key.

- **Import returns `200` for both outcomes** (`committed` discriminates) with a
  `Location` header; structural problems are 4xx `{"error","line"?,"trace_id"?}`;
  unexpected errors are a generic `500` plus `trace_id` (details only in the log).
- **Extra read endpoints beyond the brief's list:** `GET /api/lookups` (so a
  frontend never hardcodes types/statuses) and a `limit` param on search. Read
  response shapes are specified in `docs/ARCHITECTURE.md` section 3.
- **The OpenAPI spec uses base path `/` with full paths** (`/api/...`,
  `/healthz`); the earlier `/api` base path rendered `/healthz` as
  `/api/healthz`. Response fields are marked required and pointer fields
  `x-nullable` so generated client types are accurate.

## Backend structure

- **Go module path**: `smart-grid-asset-management/backend` — no remote
  git host dependency needed for local development.
- **`main.go` lives at the backend root, not `cmd/server/main.go`.**
  The `cmd/<binary>/` convention exists to disambiguate multiple
  entry points in one module; this project only ever builds one binary
  (no separate migrate/worker/CLI command — migrations are the
  compose `migrate` service's job), so the extra directory level
  wasn't earning its keep.
- **Layered architecture**: `router` → `handler` → `service` →
  `repository`, with `entity/dto` (API wire types, snake_case) and
  `entity/model` (DB-shaped domain types) kept separate under one
  `entity/` namespace, plus `config` (env parsing) and `base` (infra
  bootstrap) underneath everything. `entity/` itself holds no files
  directly — same pattern as `base/` (see below). See each package's
  doc comment (top of its main file) for its exact responsibility.
- **gin router** (originally stdlib `http.ServeMux`; see the "gin + GORM"
  entry below): method+path routing (`GET /api/assets/:assetId`, `c.Param`)
  with literal segments winning over parameters (`/assets/roots`,
  `/assets/search` vs `:assetId`), `HandleMethodNotAllowed` for a 405 with an
  `Allow` header, and trailing-slash/fixed-path redirects turned off so only
  the exact documented paths exist.
- **`config` package**: every environment variable is read and
  validated once, in `config.Load()`, instead of scattered `os.Getenv`
  calls — `main.go` fails fast with a clear error if a required var is
  missing, rather than failing confusingly several layers down.
  - **Discrete `DB_HOST`/`DB_PORT`/`DB_USER`/`DB_PASSWORD`/`DB_NAME`/`DB_SSLMODE`**,
    not a single `DATABASE_URL` — each field is legible on its own in
    `.env`/CI, and a password never needs URL-escaping. `config.DBConfig.DSN()`
    assembles the libpq connection string internally.
- **`router` package**: every HTTP path is registered in one place
  (`router.New`), separate from `main.go`'s wiring and from the
  handlers themselves — the full set of endpoints (and what guards
  each one, e.g. the API-key middleware) is readable at a
  glance.
- **`middleware` package**, adapted from the reference project's Gin
  middleware (each is a `gin.HandlerFunc`, or returns one):
  - `SecurityHeaders` — the same defensive-header idea, but scoped down
    (CSP's `connect-src`/`img-src`/etc. are `'self'`, not wildcarded —
    this app has no third-party origins to allow) and with the
    IE-era/legacy headers (`X-XSS-Protection`, `X-Download-Options`,
    `X-DNS-Prefetch-Control`) dropped as dead weight.
  - `CORS` — wildcard by default, since this app has no
    cookies/credentials anywhere to make that risky; set
    `CORS_ALLOWED_ORIGINS` to restrict it.
  - `RateLimiter` — an in-memory, per-(client IP, path) fixed-window
    limiter (100 req/10s by default). Explicitly process-local, no
    shared store — the right trade-off for a single backend instance,
    called out here so it's not mistaken for something that'd survive
    horizontal scaling unchanged.
  - `RequestSizeLimit` — caps every request body (10 MiB default, see
    `router.MaxRequestBytes`). The cap surfaces as a read error only
    once a handler reads past it, so a handler that decodes a capped
    body must check `middleware.IsBodyTooLarge(err)` itself.
  - `RequireAPIKey` — guards every `/api` endpoint with the shared key.
  - `TraceID` — reads/generates an `X-Trace-Id` header and stamps it on
    the request's `context.Context` so every log line for a request can
    be correlated (see the `base/logs` entry below). Registered first
    in the chain so it's in context for every other middleware/handler.
- **`internal/base`** holds cross-cutting bootstrap code with no
  business/HTTP knowledge — mirroring the reference project's
  `base`/`init` split. It holds no files directly, only subpackages,
  one per concern:
  - **`base/logs`** was ported from an existing internal Go utility
    library, then trimmed hard: no Kafka/Redis sinks, no gRPC trace-ID
    extraction, no file rotation, no adapter registry — just four
    levels (`Debug/Info/Warn/Error`) writing to one output. Those
    features weren't used anywhere in this project; keeping them would
    have added `kafka-go`, `go-redis`, and `google.golang.org/grpc` as
    dependencies for no reason. All comments were translated from the
    source's Chinese to English during the port.
    - **[ASSUMPTION] Re-added plain HTTP trace-ID correlation, on top
      of `log/slog`.** Once a real request pipeline (`/api/imports`
      etc.) starts logging per-row outcomes, "which request produced
      this line" becomes a real question — unlike the gRPC extraction
      above, which stays cut since nothing here speaks gRPC. `base/logs`
      now wraps the standard library `log/slog` (`WithCtx(ctx).Warn(...)`)
      instead of hand-rolling structured fields + context plumbing on
      the old printf logger: it's stdlib, needs no new dependency, and
      is less code to own. A small `slog.Handler` pulls the trace ID
      out of `context.Context` (stashed there by `middleware.TraceID`,
      via an unexported struct key — not the raw string key the
      reference project used, which `go vet`/`staticcheck` flag as
      collision-prone) and adds it as a `trace_id` field on every line.
      Output is `slog.NewTextHandler` (logfmt-style), not JSON, for
      readability in `docker compose logs`.
  - **`base/postgres`** (`postgres.New(ctx, postgres.Config{...})`) is
    a deliberately small stand-in for the reference project's
    gorm+retry+mTLS connection setup: it returns a `*gorm.DB` (pgx
    driver) with `SkipDefaultTransaction` (writes are single statements or
    inside an explicit `repository.InTx`, so GORM's implicit per-write
    transaction would only add round trips and nested savepoints),
    `TranslateError` (unique/foreign-key violations arrive as
    `gorm.ErrDuplicatedKey`/`ErrForeignKeyViolated`, which the repository
    maps to `errx.ErrConflict`) and a silenced GORM logger (failures are
    logged once, with the trace ID, by the layers above), plus a bounded
    connect-retry loop. It only connects — it does not
    apply migrations. It used to live as a single `base/db.go` file
    directly in the `base` package; split into its own subpackage for
    the same reason as `base/logs`/`base/httpx` — one concern per
    subpackage, `base/` itself as a pure namespace.
    - An earlier version also had an in-process `SyncDB` option (via
      golang-migrate) as an alternative to the compose `migrate`
      service, for running the backend directly against a bare
      Postgres. That was removed: nothing in this project's actual
      workflow used that path (`CLAUDE.md`'s dev commands center on
      `docker compose up`), so it was speculative capability with no
      real caller — exactly what `CLAUDE.md`'s own non-negotiables say
      to avoid.
  - **`base/httpx`** — now just `ClientIP`, the framework-free client
    address derivation (the last `X-Forwarded-For` hop, else the peer).
    Its `WriteJSON`/`WriteError` helpers went away with the move to gin,
    whose `c.JSON` does the same job. There is still no generic
    `{code, message, data}` envelope — response bodies are the
    `entity/dto` types themselves, per `docs/ARCHITECTURE.md` section 3.
  - **`base/csvx`** — structural CSV-upload helpers: pulling the file
    out of the multipart request and parsing it into rows tagged with
    their true 1-indexed source line number, via `encoding/csv`'s
    `Reader.FieldPos` rather than a naive counter, so blank lines
    (which `encoding/csv` silently skips) and quoted multi-line fields
    don't throw off the `csv_row` numbers `ImportRejection`/
    `dto.Rejection` rely on (`docs/ARCHITECTURE.md` section 3). It only
    catches file-level structural problems — wrong extension, empty
    file, ragged/undecodable CSV — via a small typed `Error`/
    `ErrorCode`, always returned as a plain 4xx, never `ImportResponse`'s
    `rejections[]` shape. No asset-field or hierarchy semantics live
    here; those live in `service/import.go`. It was scaffolded ahead of the import handler — same pattern as
    `entity/dto/import.go`/`entity/model/import_run.go`, already
    committed unwired. It also carries two tiny, schema-agnostic text
    helpers the import service used to own: `Row.IsBlank` (Excel's
    trailing `,,,,` rows) and `NormalizeHeader` (case/BOM/space/`-`/`_`
    folding), since neither knows what a column means.
  - **`base/parallel`** (`parallel.Chunks(ctx, n, fn)`) — the chunked
    worker pool pass 1 fans out over, extracted from `service/` because it
    has no domain knowledge: workers write disjoint indices of a caller-owned
    slice (ordered, lock-free), a panic in a worker becomes an error, and a
    cancelled context stops the remaining chunks.
  - **`base/errx`** — the errors shared across repository, service and
    handler: `ErrNotFound`, `ErrConflict` and `InputError`/`InvalidInput`.
    They used to be defined in `repository/db.go` and re-aliased in
    `service/errors.go`; one definition in `base/` means no layer imports a
    lower one just to name a sentinel. Errors that only mean something to
    the import (`service.SchemaError`, `service.ErrNoDataRows`) deliberately
    stay in `service/`, beside the code that produces them.
- **gin + GORM** (a deliberate change of the original "plain `net/http` +
  `database/sql`" stack, at the project owner's request, to match the
  reference project's conventions). What that means in practice:
  - Handlers, middleware and the router are gin (`*gin.Context`).
    Behaviour was kept, checked by replaying the same requests against the
    pre-change build: identical statuses and bodies, except that JSON
    responses now carry `charset=utf-8`, an unknown path / wrong verb is a
    JSON `{"error": ...}` (was plain text), and `HEAD` is no longer answered
    implicitly for `GET` routes. Our CORS, API-key, rate-limit and
    security-header middleware are kept rather than swapped for gin-contrib
    ones, since their semantics are specific and tested.
  - The repository uses GORM's builder for everything it can express. The
    three recursive CTEs (subtree counts, descendant counts, ancestor path)
    stay raw SQL through `db.Raw` — GORM has no `WITH RECURSIVE` builder.
    `ExistingByIDs` sends its ids as one array parameter (`= ANY(?)` with
    `pq.Array`, a `driver.Valuer` GORM will not expand): an `IN ?` list
    expands one placeholder per id and a large file (rows are unbounded by
    design) would exceed Postgres's 65 535-parameter limit; a test covers
    70 000 ids. That is the one remaining use of `lib/pq`.
  - Models carry GORM tags and a `TableName()` (the lookup tables got tiny
    `AssetType`/`OperationalStatus` models for the same reason).
    `created_at`/`updated_at` are read-only to GORM (`<-:false`) so the
    database keeps owning them (`DEFAULT now()` plus the `set_updated_at`
    trigger) rather than GORM stamping the application clock.
  - Model → API conversions are methods on the model (`Asset.ToSummary`,
    `ImportRun.ToResponse`, …), so `entity/model` imports `entity/dto`;
    `dto` must never import `model`.
  - Request DTOs (`dto.SearchRequest`, `dto.ImportIDRequest`) carry a
    `Validate()`, which the service calls: the frontend's checks are UX
    only. Checks that need reference data (is this a known asset type?)
    stay in the service.
- **Frontend API types are generated from the OpenAPI spec**, the way the
  reference project does it: swag emits Swagger 2.0;
  `swagger2openapi` converts it (openapi-typescript v7 does not read 2.0);
  `openapi-typescript` writes `frontend/src/api/schema.ts`. The generated
  file is committed, the intermediate `openapi3.json` is git-ignored, and
  `npm run check:types` fails if the committed file is stale.
  `openapi-typescript` 7.13 declares a TypeScript 5 peer while the app uses
  TypeScript 6; an npm `overrides` entry points its peer at the project's own
  TypeScript (the generator only uses the printer API, and the generated
  file type-checks under `tsc -b`).
- **The CSV import pipeline lives in one file, `service/import.go`** — the
  service, the declarative column schema, pass 1 (field validation), pass 2
  (hierarchy validation) and the insert ordering — with matching tests in
  `import_test.go`. `repository/import.go` holds the audit-table queries.
- **`internal/defined`** — the app's shared constants (search/import/upload
  limits, header/cookie names, the date format) in one leaf package.
  Private tuning of one algorithm (repository batch sizes, recursion
  depth, pass-2's colour enum) stays beside its code, and nothing under
  `base/` may import it. (`const` is a Go keyword, hence `defined`.)
- **Migration runner**: `docker-compose.yml`'s one-shot `migrate`
  service (`migrate/migrate`'s official image), applying
  `db/migrations` before `backend` starts. This is the *only* path
  that applies migrations — the backend itself assumes the schema is
  already there.

- **Concurrency is confined to where it pays** (Pass 1 across row chunks,
  overlapped pre-validation reads, the three independent reads behind
  `/children`); pass 2, the transactional insert and the recursive-CTE queries
  are deliberately sequential. Workers recover panics into errors because an
  unrecovered panic on a non-request goroutine would kill the process. See
  `docs/ARCHITECTURE.md` section 4.2.
- **`middleware.Recover` and `middleware.AccessLog`** were added (chain:
  `TraceID → AccessLog → Recover → …`) and the HTTP server has read/write/idle
  timeouts; `MAX_REQUEST_BYTES` replaced the hardcoded 10 MiB upload cap.
- **Integration tests use an isolated schema per test** (`internal/testutil`),
  activated by `TEST_DATABASE_URL` and skipped otherwise.
- **Swagger UI works in a browser**: only `/swagger/` gets a CSP that allows the UI's inline
  init script (`middleware.DocsCSP`); every other response keeps `script-src 'self'`.

## Repo layout

- **`docs/` holds every reference/deliverable doc except `README.md`
  and `CLAUDE.md`**: `ARCHITECTURE.md`,
  `AI_JOURNAL.md`, `KNOWN_LIMITATIONS.md`, and this file. `README.md`
  stays at the repo root as the entry point; `CLAUDE.md` stays at the
  repo root too, but for a different reason — that's specifically
  where Claude Code auto-loads it from, so moving it would silently
  break auto-discovery, not just tidiness.
- **One agent-instructions file, not two.** This started as both
  `CLAUDE.md` and `AGENTS.md` with identical content (to cover
  Claude-Code-specific and tool-agnostic discovery at once), but
  keeping two files in sync by hand wasn't worth it. Consolidated into
  `CLAUDE.md` alone, since this project is actually developed with
  Claude Code — its root-auto-load is the one that matters here. The
  trade-off: other tools that only check the generic `AGENTS.md`
  convention won't pick this file up automatically.

## Frontend visual identity

- **[ASSUMPTION] Dark mode is a token swap, with a header toggle.** `index.css` redefines
  the same semantic tokens under `:root[data-theme="dark"]` (plus `color-scheme: dark`); no
  component has a `dark:` class or knows the theme. The choice (light/dark) is saved in
  `localStorage` (`theme`); with none saved the OS preference decides, once, at load. A small
  inline script in `index.html` sets `data-theme` before first paint so there is no white
  flash. The two dialog overlays and the browser `theme-color` now follow the theme too
  (`--color-overlay`). In dark, `primary` is a light blue and `on-primary` is dark text on it.
  Measured contrast (WCAG AA needs 4.5:1), dark: text on surface 15.7, on surface-muted 14.3,
  on neutral-soft 12.4; text-muted on surface 7.6, surface-muted 6.9, neutral-soft 6.0;
  primary on surface 8.3, surface-muted 7.5, primary-soft 5.9; on-primary on primary 8.5,
  on primary-hover 10.7; danger on surface 8.4, on danger-soft 6.9; success on surface 10.2,
  on success-soft 7.2; warning on surface 10.7, on warning-soft 7.6. The light palette's ratios
  are unchanged (all at least 5.3:1 for text).
- **[ASSUMPTION] Nested panels use two tones, not four.** A grey card (`surface-muted`)
  holds white content (`surface`), and content inside that stays white: an expanded import
  in the history is white with a hairline above it, and its rejection table has a grey header
  row only. Alternating grey, white, grey, white inside one another read as muddled because
  the tones and borders are so close.
- **[ASSUMPTION] One spacing scale, enforced by a test.** Padding, margin, gap and
  space-x/y use only 4 / 8 / 12 / 16 / 24 / 32 px (Tailwind steps 1, 2, 3, 4, 6, 8). The
  top bar, the tree rail and the main panel share the gutters `px-4 sm:px-6 lg:px-8`;
  cards and rows use `p-4`, primary panels `p-6`. Half-steps (`py-2.5`, `gap-1.5`) and
  20/40/48 px values were replaced by the nearest step (72 sites). The one extra value is
  `pl-10`/`pr-10`, the inset that clears an icon inside an input. `src/spacing.test.ts`
  (run alone with `npm run check:spacing`, and as part of `npm test`) fails on anything else.
  Badges and icon buttons grew by a few pixels (2 px vertical padding became 4 px).

- **Brand colors are scraped, not guessed.** Sourced from a reference
  utility company's live compiled CSS bundles (frequency-ranked hex
  values pulled from the production stylesheets), not sampled by hand
  in DevTools — same "don't fake it" requirement, a scriptable method. Centralized
  as `@theme` tokens in `frontend/src/index.css`:
  `--color-brand-primary` (`#024d87`, their most prominent dark navy —
  8.7:1 contrast on white, comfortably passes WCAG AA/AAA for text),
  `--color-brand-primary-dark` (`#013964`, a programmatically darkened
  hover/active shade of the primary — not independently brand-sampled,
  since interaction-state shades are conventionally derived, not
  scraped), `--color-brand-accent` (`#38b7c5`, their most frequent
  secondary teal — decorative/accent use only, since at ~2.4:1 on white
  it fails text-contrast and must not carry small text),
  `--color-brand-accent-light` (`#afe7e8`, a light teal tint for
  badges/subtle highlight surfaces), and `--color-brand-surface`
  (`#f7fbff`, one of their several near-white blue-tinted backgrounds,
  replacing generic `bg-gray-50`).
- **`HealthBadge`'s status colors stay Tailwind's default
  `red-600`/`green-700`, not new brand success/danger tokens.** The
  scraped candidates (`#e76161` red, `#3dcc4a` green) measure ~3.3:1 and
  ~2.1:1 against white respectively — both fail WCAG AA's 4.5:1 for
  normal text, and darkening them further would mean guessing a shade
  the reference site never actually shipped, which is exactly what the
  "sample, don't guess" rule was meant to avoid. Tailwind's built-ins
  already pass AA and this is low-stakes internal status text, so
  they're left as-is.
- **[ASSUMPTION] The reference site's proprietary typeface was not
  replicated.** It's a commercial font; bundling it or a lookalike
  Google Font would add licensing risk for a purely cosmetic win. The
  frontend uses Inter (open licence, self-hosted through `@fontsource/inter`) as a
  clean stand-in; "pixel-perfect" is out of scope.

## Frontend build

- **React 19 and Tailwind 4 kept**, not the brief's React 18 and a Tailwind config
  file: the scaffold already used them, only React-18-compatible APIs are used, and
  downgrading buys nothing.
- **No MSW.** The brief allowed mocks only if the backend was not ready. Tests stub
  `fetch` with a route table instead, and the app was checked against the real API.
- **Import is its own page, `/import`, with no asset tree** (changed after review; the UI
  concept put it in the rail). The upload card and its check → review → confirm report are
  on the left and the history of past imports on the right (stacked on a phone), so what
  you are doing and what was done before are side by side. Checking a file does not
  navigate. The pending review lives in memory (with the chosen file), so a refresh
  discards it, but it survives moving to other pages; the old `/imports` address redirects.
- **[ASSUMPTION] Mobile support is minimal: it works at phone width, and no more than
  that has been done.** Layout is mobile-first: below `md` the tree becomes a drawer, the
  Import page's two cards and the Overview's donut and top-level assets stack, and wide
  tables scroll sideways inside their own box (asset IDs never wrap) rather than squeezing
  columns. Checked in headless Chrome at 390 px wide on the Overview, All assets, asset
  details and Import pages: the page itself does not scroll sideways. Not done: testing on
  real phones or tablets, landscape or in-between widths beyond the breakpoints, touch-specific
  behaviour (drag-and-drop is a desktop feature; on a phone Choose file is the way in), and
  the 44 px minimum tap size (buttons here are about 32–40 px). The automated tests run in
  jsdom, which does not lay anything out, so responsive behaviour has no automated coverage
  beyond a few class checks.
- **Existing scraped navy tokens kept** rather than the brief's placeholder green (the
  concept PDF is green). Colours, radii and the font are defined once in
  `frontend/src/index.css`; the semantic tokens (surface, text, border, status) were
  added beside the existing `brand-*` ones. Inter is self-hosted via `@fontsource/inter`.
- **Contrast**, measured (WCAG relative luminance) for each pair a component uses:
  text on surface 16.6:1, muted text on surface 6.24 / on muted surface 5.80 / on neutral
  chip 5.41, primary on surface 8.70 / on its tint 7.40, white on primary 8.70, success
  on its tint 5.68, warning on its tint 5.65, danger on its tint 5.59. All above 4.5:1.
  The accent teal is never used for text.
- **Descendant count badge** is shown on any root (parent-less) node, not on a hardcoded
  type, and equals `subtree_count - 1` (descendants, excluding the node itself).
- **Contents by type** (brief 4.3, counts beneath an asset) is one row of cards on the details page,
  "Contents by type, at every level", from `descendant_counts` and `groups` on the children response (one
  request, shared with the children table). Each card shows the total of that type at every depth and how
  many are direct ("0 direct · 10 deeper"). It replaced an earlier design with two rows of cards (immediate
  counts, then counts beneath), which repeated the same numbers. A Beneath column on the children
  table was tried and removed at the owner's request (2026-09-21): the cards already carry the counts. **[ASSUMPTION]** Every type that *can* occur beneath the asset
  shows a card, with 0 when there are none, because the brief names four types and a missing card reads as
  "unknown" rather than "none". Which types can occur is derived from the parent rules in `GET /api/lookups`
  (transitively, `descendantTypes` in `frontend/src/lib.ts`), not from a list of type names, so a new type or
  pairing needs no frontend change. An asset with no children shows the empty message instead of cards; if
  the lookups fail, the cards fall back to the types the API returned. "Grouped by type" for the immediate
  children is met by the cards' direct counts and the table's default sort by type.
- **Type icons and short codes** (SUB, TX, LV, SWB, PNL) come from one map in
  `frontend/src/api/assetTypes.ts`; an unknown type from the lookup table falls back to a
  derived label and a generic icon. The icons (lucide) are Building2 for a substation, Zap for a
  transformer, Rows3 for an LV board (rows of feeder breakers), Columns3 for a switchboard (a line-up of
  cubicles) and SquarePower for a switchboard panel (one switch unit). The last three replaced Cable,
  ToggleRight and PanelTop on 2026-09-21, chosen from three proposed sets, because the old ones did not
  suggest the equipment.
- **[ASSUMPTION] The footer is the end of the page, not a pinned bar** (`AppFooter` and `AppVersion`; modelled
  on the screenshot the owner supplied, and the owner preferred this layout on 2026-09-21 over a bar fixed to
  the bottom of the screen). The app name, version and "© <current year> Nicholas Ong. All rights reserved."
  (the holder named in `LICENSE`) sit at the bottom of the asset-tree sidebar and of the mobile drawer. The
  links to the API docs (`/swagger/index.html`, new tab) and `/healthz`, and a clock that ticks every second in
  the browser's own time zone with its abbreviation (for example `2026-09-21 11:18:37 GMT+8`), sit at the bottom
  right of the main panel, as its last element, so they are seen by scrolling to the end (or straight away
  when the page is short). Import has no sidebar, so the version and copyright move to the left of its footer.
  The clock is not an `aria-live` region, so a screen reader is not interrupted every second. The version is
  `version` in `frontend/package.json` (1.0.0), injected at build time as `__APP_VERSION__`, so it is bumped in
  one place; it is the app's version, separate from the API's `@version 1.0` in the Swagger spec.
- **[ASSUMPTION] nginx (and the Vite dev proxy) also forward `/swagger/` and `/docs` to the backend**, so the
  footer's docs link works from the app's own origin (8081) and needs no second port. The docs stay open
  (no key added), as before. Swagger's "Try it out" then calls `/api` on port 8081, where nginx adds the key, so
  it works without pressing Authorize; on port 8080 the key is still entered by hand.
- **Tree keyboard**: arrows, Home/End, Right/Left, Enter/Space, with a roving tabindex.
  Selecting a row also expands it; the chevron only toggles.
- **Deep links** reveal the asset by fetching `/ancestors` for the route parameter, so
  refresh, back/forward and the search flow share one code path.

## Changes of 2026-09-20 (real data, delete, activity)

- **Deleting one asset is added although the brief lists it as out of scope** ("Editing and
  deleting individual assets"), at the owner's explicit request. It is refused (409, with the
  child count) while the asset has children. It requires the shared `API_KEY` as an
  `x-api-key` header (fails closed when no key is configured); the owner's intent was that
  the *request* carries the key, not that a user types one, so nginx and the Vite dev proxy
  add it and the frontend never holds it. (It first guarded only DELETE; see the next entry.) Create-one and update stay out:
  assets are never overwritten or edited, and a duplicate `asset_id` is rejected.
- **Every `/api` endpoint needs the shared API key; only `/healthz` and the Swagger docs are
  open.** Added at the owner's request (2026-09-20, S11) after the key had guarded only DELETE. It is
  one `RequireAPIKey` on the `/api` route group, placed after the rate limiter so guessing is
  throttled. It fails closed: with no `API_KEY` set every `/api` call is 403 (the backend logs a
  warning at start-up). nginx and the Vite dev proxy now add the header to every proxied `/api`
  request, not just DELETE, replacing anything the client sent. Every operation carries
  `@Security ApiKeyAuth`, and a test fails if one does not. This is a shared secret, not user
  authentication; see `KNOWN_LIMITATIONS.md`.
- **There is no delete-all** (it was built and then removed at the owner's request). Starting
  over means `docker compose down -v`.
- **Deletions are logged, not recorded in the database** (`asset deleted`, with client IP and
  trace ID), so the Import activity page, which lists imports only, does not show them.
- **Preview writes no audit row**: only a confirmed commit (even one that stores nothing)
  appears in Import activity.
- **The template's example rows are fictional** (`EXAMPLE-SUB-001` …) and must be deleted
  before importing real data; its header is generated from the importer's own column table
  so it cannot drift.
- **Trace IDs are 8 hex characters** (4 random bytes) instead of a UUID: they only have to
  tell apart requests in flight around each other. A client-supplied `X-Trace-Id` is still
  echoed unchanged.
- **Container health checks** poll every 2 s while starting and every 5 minutes after
  (`--start-interval`), instead of every 10 s, to keep the log quiet without slowing start-up.
- **`backend/testdata/grid_assets.csv`** is the supplied file, committed so the fixture test
  runs instead of skipping.
- **Layout uses the width it is given.** The rail is 22 / 26 / 28 rem at md / lg / xl, the
  main panel is left-aligned and fluid (max 110 rem) instead of a centred 72 rem column that
  left a blank area on wide monitors, and asset details are two columns from xl. Vite writes
  the bundle to `static/` because the app has a `/assets` route.

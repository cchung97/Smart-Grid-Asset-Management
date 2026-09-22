# CLAUDE.md — Architecture Conventions & Dev Guide

This file orients any AI agent (or human) working in this repo. See
`docs/ARCHITECTURE.md` for the full technical spec — this file is the condensed
"how to work here," not a restatement of it. See `docs/ASSUMPTIONS.md` for every
judgment call made along the way.

## Non-negotiables

- **Backend re-validates everything the frontend also checks.** The
  frontend's CSV/form validation is UX only — never trust it as the
  source of truth. Every rule in `docs/ARCHITECTURE.md` section 4 must be
  enforced server-side regardless of what the client already checked.
- **Import is two-step and transactional.** `POST /api/imports/preview`
  validates and writes nothing. `POST /api/imports` validates again (never
  trusting the preview) and, in one DB transaction, stores the rows that pass plus
  the `import_runs`/`import_rejections` audit rows; rejected rows and anything under
  a rejected parent are skipped and reported. If the caller sends the preview's
  `expected_fingerprint` and the outcome changed, nothing is written (412). Existing
  assets are never updated: a duplicate `asset_id` is rejected. See
  `docs/ARCHITECTURE.md` section 4.
- **No user accounts, no RBAC, no edit; one shared API key.** The API is import-first. At the
  owner's request (2026-09-20, outside the brief's scope, recorded in `docs/ASSUMPTIONS.md`) it
  also has read-only list/stats endpoints and one delete: an asset with no children (409
  otherwise). There is deliberately no delete-all. **Every `/api` endpoint requires the shared
  `API_KEY` as `x-api-key`** (`middleware.RequireAPIKey` on the `/api` group; 401 if wrong, 403
  with no key set, so it fails closed); only `/healthz` and the Swagger docs are open. The
  frontend never holds the key: nginx (Docker) and the Vite dev proxy add it to every `/api`
  request. A new `/api` route is protected by being in that group, and needs `@Security
  ApiKeyAuth` (`router/swagger_test.go` checks). Do not add delete-all, create-one,
  update/edit endpoints, a login screen, or user accounts "for realism."
- **Frontend talks only to the Go API**, never directly to Postgres. nginx (and the Vite proxy) forward
  `/api`, `/healthz` and the open Swagger docs (`/swagger/`, `/docs`); only `/api` gets the key.

## Dev commands

```bash
# Full stack (db -> migrate -> backend -> frontend)
docker compose up --build

# Backend tests (database tests skip unless TEST_DATABASE_URL is set; they
# use an isolated schema per test, so dev data is never touched)
cd backend && go test -race ./...
cd backend && TEST_DATABASE_URL='postgres://user:pass@localhost:5432/db?sslmode=disable' go test -race ./...

# Regenerate the OpenAPI spec after changing handler annotations/DTOs,
# then the frontend's generated API types from it (commit both)
cd backend && make docs
cd frontend && npm run generate:types
cd frontend && npm run check:types   # fails if src/api/schema.ts is stale

# Frontend tests / build
cd frontend && npm test
cd frontend && npm run build
```

There is no CI/CD pipeline (local build and run only, see `docs/ASSUMPTIONS.md`): run the tests by hand.
Build, run, update and troubleshooting commands are in `docs/DEPLOYMENT.md`.

Copy `.env.example` to `.env` before running `docker compose up` for the
first time.

## Stack

- **Backend**: Go with `gin` (HTTP) and `gorm` (Postgres 16, pgx driver);
  recursive CTEs stay raw SQL via `db.Raw`. This replaced the original
  plain `net/http` + `database/sql` stack — see `docs/ASSUMPTIONS.md`.
- **Frontend**: React + TypeScript (Vite), React Router, TanStack Query.
  No Redux/Zustand — not justified at this scale.
- **DB**: Postgres 16, migrations in `db/migrations/`, applied by the
  `migrate` one-shot service in `docker-compose.yml`.

## Backend conventions

- **Layering**: `router` → `handler` → `service` → `repository`. Handlers bind,
  call a service and write JSON — no business logic, no SQL. `base/` is generic
  and must not import app packages; `defined/` is a leaf every layer may import.
- **Constants** shared across packages (limits, header/cookie names, formats)
  live in `internal/defined`; private tuning of one algorithm stays beside it.
- **Models** carry GORM tags, a `TableName()`, and their model → dto helpers
  (`Asset.ToSummary`, …). `dto` never imports `model`.
- **Request DTOs** (`dto.SearchRequest`, `dto.ImportIDRequest`) have a
  `Validate()` that the *service* calls, so the server re-validates regardless
  of the client. Rules needing reference data stay in the service.
- **Errors**: shared sentinels and `InputError` are in `base/errx`;
  import-specific ones (`service.SchemaError`, `service.ErrNoDataRows`) stay in
  `service`. `handler/errors.go` is the only place that maps errors to statuses.
- **Repositories use GORM's builder**; raw SQL only where GORM cannot express it
  (the recursive CTEs). Keep large id lists as one array parameter.
- **API changes**: update the swag annotations, run `make docs`, then
  `npm run generate:types`. `router/swagger_test.go` fails if a route and the
  spec disagree.

## Operational notes

- Trace IDs are 8 hex characters (`X-Trace-Id`, also in every log line); a client-sent
  one is echoed unchanged.
- Container health checks poll every 2 s while starting (so `depends_on:
  service_healthy` is quick) and then every 5 minutes.
- Dates in imports: `YYYY-MM-DD`, `D/M/YYYY` or `D/M/YY`, always day-first.

## Frontend conventions

- **One shell, two layout routes.** `AppShell` is layout only (top bar, optional rail with `AppVersion` at its foot, and `AppFooter` as the last element of the scrolling main panel, not a fixed bar). `ExplorerLayout` (tree
  rail + `<Outlet/>`) serves the explorer; `PageLayout` (no rail) serves `/import`. Both sit
  under `AppRoot`, which owns the client state. Selecting an asset never leaves the tree.
- **Server state is TanStack Query only.** Keys and fetchers live in
  `src/api/queries.ts`; the only client state is `src/state/` (expanded tree nodes,
  the two-step import flow with its chosen `File`, the mobile drawer). Do not add a store.
- **Import UI is check → review → confirm**, all on the `/import` page (upload card and report on the
  left, history on the right, stacked on a phone; no rail, no navigation on check). `/imports` redirects there. `state/useImportFlow.ts` owns the
  stages; a 412 on confirm re-checks the same file, a 409 offers "Check again".
  Once a file is chosen the dropzone is replaced by a chip with a remove button.
- **Tokens only.** Colours, radii and the font are defined once in
  `src/index.css`; components use the utilities (`bg-surface`, `text-text-muted`),
  never a hex code. Dark mode is the same token names redefined under
  `:root[data-theme="dark"]` in `index.css`, so add every new colour token to both, and check
  WCAG AA (4.5:1) for any new text/background pair in both themes.
- **Per-panel states.** Tree, details, search and import each render their own
  loading/empty/error state (`components/ui/`), never one global spinner.
- **Accessibility is not optional**: tree roles and `aria-*` in sync with state,
  keyboard operable, `:focus-visible` never suppressed. Give a tree row an
  `aria-labelledby` (its own label) so `aria-owns` does not pull its subtree into its name.
- **Tests stub `fetch`** with `src/test/utils.tsx` (`mockApi`, `renderApp`) and assert
  call order; no MSW.
- **Layout is mobile-first.** Below `md` the rail is a drawer (Radix Dialog) that
  closes on navigation; wide tables scroll sideways and keep the Asset ID column on one line.

## Definition of done (per feature)

- Backend: table-driven unit test(s) covering the happy path and at
  least one rejection/edge case; integration test against the real `db`
  container where the feature touches the database.
- Frontend: explicit loading/empty/error states for any panel that
  fetches data — not just a page-level spinner.
- Any deviation from the brief or `docs/ARCHITECTURE.md` is marked
  **[ASSUMPTION]** in `docs/ARCHITECTURE.md` (or noted in
  `docs/KNOWN_LIMITATIONS.md` if it's a trade-off rather than a design
  choice) — don't make silent judgment calls.

## Every new execution → update README.md and AI_JOURNAL.md

Every time a new piece of work is executed (a step, feature, restructure, or
notable fix — not a pure question/answer), finish by updating both files
so they never lag behind the code:

- **`README.md`** — bring it in line with what now exists: run/test commands,
  API endpoints, backend layout, stack, and any status or setup changes.
  Remove anything the change made stale.
- **`docs/AI_JOURNAL.md`** — record what was done and how AI was used:
  update the Summary (delivered / not delivered), and add to the relevant
  numbered sections (sequencing, representative prompts, rejected or revised
  suggestions, verification, where the agent struggled, known gaps and next
  steps). Keep its factual claims (session/test counts, what is delivered)
  accurate, and write in the same first-person voice as the existing entries.

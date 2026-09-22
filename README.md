# Smart Grid & Asset Management

CSV asset import/validation and asset hierarchy explorer for smart grid
equipment (substations, transformers, switchboards, etc.).

## Stack

Go — gin + GORM (backend) · React + TypeScript, Vite, Tailwind, React Router, TanStack Query (frontend) · PostgreSQL 16 · Docker Compose

## Prerequisites

- Docker and Docker Compose

## Running it

```bash
cp .env.example .env   # then fill in your own API_KEY (see comment in the file)
docker compose up --build
```

This brings up, in order: `db` (Postgres) → `migrate` (applies
`db/migrations/`) → `backend` (Go API) → `frontend` (nginx serving the
React app, proxying `/api/*` and `/healthz` to the backend).

If you've previously run `docker compose up` against an older copy of
this repo, run `docker compose down -v` first — the initial schema
migration was edited in place (no prior real data existed to preserve),
and `migrate` won't re-apply a version it already recorded as applied
against a stale `pgdata` volume.

- Frontend: http://localhost:8081
- Backend health check: http://localhost:8080/healthz (also http://localhost:8081/healthz)
- Swagger docs: http://localhost:8081/swagger/index.html, linked as **API docs** in the app's footer, or directly
  at http://localhost:8080/swagger/index.html (open, no key needed; `/swagger/doc.json` and
  `/swagger/swagger.yaml` are the machine-readable spec). On port 8081 the page can call the API straight away
  (nginx adds the key); on port 8080 press **Authorize** and enter the `API_KEY` from `.env`.

Build, run, update and troubleshooting commands (and how to run the tests) are in
[`docs/DEPLOYMENT.md`](docs/DEPLOYMENT.md). There is no CI pipeline: only a local build and run is needed.

## Using the explorer

Two layouts share one top bar (navigation, search and an **Import** button that is on every
page). The explorer pages (**Overview** `/`, **All assets** `/assets`, one asset
`/assets/:assetId`) add a left rail that holds only the asset tree and a main panel; on a
phone the rail opens as a drawer from the **Assets** button. **Import** (`/import`) is a
separate full-width page with no tree. `/imports` redirects to `/import`. Links, refresh
and back/forward all work.

- **Import** (`/import`) is one page with two labelled cards side by side: the upload card
  on the left (with the report of the file in play under it) and the import history on the
  right; on a phone they stack in that order. The page uses the full width, and on screens
  wider than 1536 px the two cards split 50/50 so the history's rejection table is not cut off.
  It is check → review → confirm.
  Choose or drop a `.csv` (the dropzone is replaced by the chosen file with an **X** to
  remove it), press **Check file**, and the page shows how many rows can be imported and
  which are rejected and why. **Nothing is saved yet.** Then **Import N valid rows** stores them (rejected rows are skipped), or
  **Cancel**, fix the file and check it again. The server re-validates on confirm and
  refuses if the result changed since you looked. Assets that already exist are never
  overwritten. **Download template** gives a CSV with every column and example rows;
  **Download report** saves the rejected rows as CSV.
- **Footer**: the app name, version (`v1.0.0`, from `frontend/package.json`) and copyright are at the bottom of the
  asset-tree sidebar (in the drawer on a phone; on Import, which has no sidebar, at the left of the footer). At the
  end of the page, after you scroll down, **API docs** and **Health** links sit at the bottom right beside a live
  clock in your browser's time zone. It is not a bar pinned to the screen.
- **Dark mode**: the moon/sun button at the right of the top bar switches theme; the choice
  is remembered in the browser, and the first visit follows the OS setting.
- **Overview**: totals by type, the top-level assets, and beside them (above them on a phone) a donut of the operational status split with a legend of exact counts and percentages (each row has an icon and a readable label; hover a slice or a row to see its count in the centre). Navigation is
  in the top bar only; the page does not repeat it.
- **All assets**: every asset, filterable by text, type and status, paged by the server. Click
  a column heading (Asset ID, Name, Type, Status, Parent) to sort; the server sorts, so it orders
  every page, not just the one on screen. Click again to reverse. Assets with no parent sort
  last. The sort is kept in the URL (`?sort=name&dir=desc`).
- **Asset details**: fields; **Contents by type, at every level**, one card per type that can sit under it
  (a substation: transformers, LV boards, switchboards and switchboard panels), with the total at every depth,
  how many are direct, and 0 where there are none; a table of the immediate children
  sortable by name, type, status, rating or commissioned date (click a header; the arrow
  shows the direction; blanks sort last; the choice is kept in the URL), and **Delete asset**
  (blocked while it has children). Statuses read as "In service", not `IN_SERVICE`.
- **Asset tree**: roots load first, children when first expanded; the icon button in its
  header collapses every open node. Keyboard: Tab into the
  tree, arrows to move, Right/Left to expand/collapse, Enter to select.
- **Search** (top bar, debounced 300 ms): arrows to move, Enter to open, Escape to close.
  Opening a result expands its ancestors in the tree, then navigates to it.
- **Import history** (right of `/import`): past imports, newest first, each with rows
  imported, rejected and outcome; open one to see who uploaded it and its rejected rows. The import you just confirmed is marked.
  Imports only: deletions are not listed here.

## API

Full request/response schemas, status codes and examples are in the Swagger
docs above. Every endpoint below needs the `x-api-key` header (see the note after the table). The spec in `backend/docs/` is the contract: the frontend's API
types (`frontend/src/api/schema.ts`) are generated from it, never hand-written.

| Method | Path | Purpose |
|---|---|---|
| `POST` | `/api/imports/preview` | Validate a CSV (multipart field `file`) and report what would be imported or rejected; stores nothing |
| `POST` | `/api/imports` | Validate again and store the rows that pass (optional field `expected_fingerprint` from the preview; 412 if the result changed) |
| `GET` | `/api/imports` | Import activity, newest first (`limit`, `offset`) |
| `GET` | `/api/imports/template` | Download a CSV template with example rows |
| `GET` | `/api/imports/{importId}` | Re-fetch a past import's result and rejected rows |
| `GET` | `/api/lookups` | Asset types, statuses and parent/child pairings validation uses |
| `GET` | `/api/assets` | All assets, paged (`q`, `type`, `status`, `sort` = `asset_id`/`name`/`type`/`status`/`parent`, `dir` = `asc`/`desc`, `limit`, `offset`) |
| `GET` | `/api/assets/stats` | Counts by type and status |
| `GET` | `/api/assets/roots` | Top-level assets with child/subtree counts |
| `GET` | `/api/assets/{assetId}` | One asset's details |
| `DELETE` | `/api/assets/{assetId}` | Delete one asset; 409 if it still has children |
| `GET` | `/api/assets/{assetId}/children` | Children grouped by type (each with rating and commissioned date), with subtree and per-type descendant counts |
| `GET` | `/api/assets/{assetId}/ancestors` | Path from the top of the hierarchy to the asset |
| `GET` | `/api/assets/search?q=&type=&limit=` | Search by id or name |

CSV columns are matched by header name in any order (`asset_id`, `asset_type`,
`asset_name`, `operational_status` required; `parent_asset_id`, `voltage_kv`,
`rating_kva`, `manufacturer`, `model`, `serial_number`, `commissioned_date`
optional); unknown columns are ignored and reported. `commissioned_date` may be
`YYYY-MM-DD`, `D/M/YYYY` or `D/M/YY` (always day first, so `12/3/10` is 12 March
2010). Rejected rows are reported with their line in the uploaded file and written
to the server log. The API has no user accounts (out of scope), but **every `/api` endpoint needs the
server's `API_KEY`** (from `.env`) as an `x-api-key` header; only `/healthz` and the Swagger docs are
open. The frontend's nginx adds the key to every proxied request (the Vite dev proxy does the same),
so nobody types it and it never reaches the browser; a direct call to the backend port without it
gets 401, and with no `API_KEY` set every `/api` call gets 403. Example:
`curl -H "x-api-key: $API_KEY" http://localhost:8080/api/assets/stats`. There is no "delete all": to
start over, run `docker compose down -v`. See `docs/KNOWN_LIMITATIONS.md`.

## Backend layout

```
backend/
  main.go            wiring and graceful shutdown
  docs/              generated Swagger spec (make docs), served at /swagger
  internal/
    router/          every route and the global middleware chain, in one place
    middleware/      gin middleware: trace ID, access log, recover, security headers,
                     CORS, rate limit, size limit, API key (`/api`)
    handler/         gin handlers: bind the request, call a service, write JSON;
                     one place maps errors to status codes
    service/         business logic: two-pass CSV import (import.go), asset queries, lookup cache
    repository/      GORM queries (recursive CTEs via db.Raw) and InTx
    entity/model/    GORM models, TableName(), and model -> dto conversions
    entity/dto/      API wire types; request types carry Validate()
    defined/         shared constants (limits, header names, date format)
    config/          environment parsing
    base/            generic, app-agnostic helpers: csvx, errx, httpx, logs, parallel, postgres
    testutil/        isolated-schema test database
```

## Frontend layout

```
frontend/src/
  api/          generated schema.ts/types.ts, fetch client, TanStack Query hooks, asset-type icons
  components/   AppShell (top bar, optional rail, mobile drawer, footer), AppRoot (client-state provider),
                ExplorerLayout (tree rail) and PageLayout (no rail) route layouts, AppFooter (docs and health links, clock; AppVersion is the version and copyright),
                ImportPage = ImportPanel (upload card) + ImportFlowPanel (ImportPreview/ImportResult/RejectionTable)
                + ImportHistory, HomePanel (overview), AllAssetsPanel,
                AssetTree/AssetTreeNode, AssetDetailsPanel, SearchBar,
                ui/ (empty, error, skeleton, badges, confirm dialog, pagination)
  state/        expanded-tree reducer, the two-step import flow (client state; the only other state is the drawer)
  index.css     design tokens (colours, radii, font) — no hex codes elsewhere
```

## Development

```bash
# Backend
cd backend && go test -race ./...   # unit tests; database tests skip themselves
cd backend && make docs             # regenerate Swagger docs after changing handler annotations/DTOs
                                    # (needs the swag CLI: make swag-install)

# Backend database integration tests (isolated schema per test; dev data untouched)
docker compose up -d db
cd backend && TEST_DATABASE_URL='postgres://<user>:<password>@localhost:<POSTGRES_PORT>/<db>?sslmode=disable' go test -race ./...

# Frontend
cd frontend && npm run generate:types  # regenerate src/api/schema.ts from backend/docs/swagger.json
cd frontend && npm run check:types     # fails if schema.ts is out of date with the spec
cd frontend && npm test                # Vitest + React Testing Library (includes the spacing-scale check)
cd frontend && npm run check:spacing   # only the spacing-scale check: fails on off-scale padding/margin/gap
cd frontend && npm run lint
cd frontend && npm run build
```

There is no CI pipeline; run the commands above by hand before a release (see `docs/DEPLOYMENT.md`).

After changing an endpoint or DTO: `make docs` (backend), then `npm run generate:types`
(frontend), and commit both generated outputs.

**Implementation report and user manual.** [`docs/Smart-Grid-Implementation-and-User-Manual.pdf`](docs/Smart-Grid-Implementation-and-User-Manual.pdf)
(44 pages, version 1.2): functional requirements with a traceability matrix, architecture, ER, backend class and sequence
diagrams, the validation algorithm, API reference, testing, a step-by-step user manual with
screenshots, and a condensed AI usage journal chapter. Its sources are in `docs/manual/` (`content-*.html`, Mermaid diagrams in `diagrams/`,
screenshots in `shots/`; that folder is git-ignored, so the sources stay local and only the PDF is committed); rebuild with `cd docs/manual && npm install && node build.mjs` (needs a
Chromium; set `CHROME=/path/to/chrome` if Playwright's is not installed). `capture-screenshots.mjs`
retakes the screenshots against an empty stack that you start on other ports, for example `docker compose -p manualshots up --build -d` with `FRONTEND_PORT=18081`, the backend published on 18080 and no host port on the database (the script expects `http://localhost:18081`). Version 1.2 covers the footer, the new type icons, the docs link through nginx, the removal of the CI pipeline, and the AI usage journal chapter.

See `CLAUDE.md` for conventions and non-negotiables,
[`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md) for the full technical
spec, and [`docs/ASSUMPTIONS.md`](docs/ASSUMPTIONS.md) /
[`docs/KNOWN_LIMITATIONS.md`](docs/KNOWN_LIMITATIONS.md) for every
judgment call and trade-off made along the way.

## License

Proprietary — All Rights Reserved. See [`LICENSE`](LICENSE).

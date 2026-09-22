# AI Journal

A record of how AI was used to build this project, reconstructed from the Claude
Code session transcripts and the git history. All times are local (UTC+8) on
2026-09-20. Session IDs (8 characters) refer to the transcripts, which are
available on request.

## Summary

- **Tool and model**: Claude Code (VS Code extension), Claude Sonnet 5 in every recorded
  session (S1–S11).
- **Effort**: about three hours hands-on inside a 3.6-hour window (14:40–18:16),
  across eight sessions, 69 recorded human inputs (instructions, mid-run
  redirections and answers to the agent's questions) and 14 sub-agent runs.
- **How the work was organised**: one concern per session, several running side by
  side; the agent researches first (read-only sub-agents), proposes a plan, and I
  decide before anything is built; a project I already maintain (the *reference
  project*) supplies proven patterns; every judgment call is written into
  `docs/ASSUMPTIONS.md`.
- **Delivered**: the full-stack scaffold and documentation set, the backend API
  (CSV import with two validation passes and an all-or-nothing commit, hierarchy and
  search endpoints, Swagger), a restructure onto gin + GORM, and frontend API types
  generated from the Swagger spec.
- **Frontend and a second round (S9, 2026-09-20 18:25–20:50)**: the explorer UI from the UI concept PDF (single-page shell, lazy asset tree,
  details, type-ahead search, per-panel loading/empty/error states), then, after I
  imported the real data and saw every row rejected, a date fix, a check → review →
  confirm import that replaced the all-or-nothing policy, an overview, an all-assets list,
  an import-activity page, a template download, delete one (a delete-all was built and then
  removed at my request), a mobile drawer, 8-character trace IDs and a 5-minute health check.
  About 120 Go tests (with a real Postgres) and 96 Vitest tests. Sessions S9 onwards are
  not included in the session, input and sub-agent counts above or in section 7; see the
  S9 to S11 entries in 2.3.
- **Review refinements (S10, 21:00–22:14)**: the Import page and sidebar cleanup, tree
  collapse-all, sortable tables, readable status labels, the spacing scale, dark mode and a
  status donut. All seven requested layout changes are in.
- **Closure check and documentation alignment (S11)**: a gap check on everything except
  deployment. It found and fixed a 500 on a malformed upload, a truncated asset id on the
  overview cards, a CI pipeline that skipped the frontend tests, the types check and the
  database tests, and documentation that had drifted.
- **Implementation report and user manual (2026-09-21)**: a PDF (`docs/Smart-Grid-Implementation-and-User-Manual.pdf`; 43 pages in v1.1, 41 in v1.2 after S12, 44 after the S13 documentation refresh)
  with the functional requirements from brief 4.1–4.3 traced to code, architecture, ER, backend class and
  sequence diagrams (no frontend components, at my request), the validation algorithm, API reference and a
  user manual with screenshots taken from a clean, isolated Compose stack. Writing it turned up one gap
  against the brief (per-type counts beneath a substation were returned by the API but not shown); I asked
  for it to be fixed, and it now is (`AssetDetailsPanel`, 12 new tests, 108 Vitest tests in total; the Beneath
  column and its tests were later removed, leaving the cards). Not
  counted in the session, input and sub-agent figures above.
- **Local-only deployment, new icons and a footer (S12, 2026-09-21)**: at my request the never-run CI
  pipeline and its variables script were removed entirely, because only a local build and run is needed; this is
  recorded as an assumption and the commands are in the new `docs/DEPLOYMENT.md`. The LV board, switchboard and
  switchboard panel icons were replaced (I chose from three proposed sets), and there is now a footer: the
  version (v1.0.0) and copyright at the bottom of the sidebar, and links to the Swagger docs (through nginx) and
  the health check beside a live clock at the end of each page. Vitest is now 112 tests; the 119 top-level Go
  tests (167 subtests) all pass with a real Postgres, none skipped. The PDF was updated afterwards, at my
  separate request (v1.2, 41 pages).
- **Documentation cleanup and AI journal chapter (S13, 2026-09-22)**: every remaining "GitLab" reference in the
  docs and the PDF sources was genericised to "CI pipeline"/"CI file", consistent with the pipeline's removal in
  S12. The PDF gained a new condensed chapter 15 (AI usage journal): tools and effort, how the work was
  organised, the session timeline, and the suggestions/decisions table, with a pointer back to this file for the
  full detailed log and representative prompts, which were left out. 44 pages, still version 1.2.
- **Not delivered**: a CI/CD pipeline or any remote deployment, by design (section 8).
- **Verified by**: repeated `go test` runs (57, 26 of them with `-race`) against a
  real Postgres, end-to-end runs through Docker Compose with `curl`, and a 37-request
  replay of the old and new servers.

## 1. Tools and models

| Tool | Model | Used for |
|---|---|---|
| Claude Code (VS Code extension), plan mode | Claude Sonnet 5 | Everything: scaffold, documentation, backend, refactors, generated types, this journal |

- **Sub-agents**: 14 runs, all Claude Sonnet 5. *Explore* agents did read-only
  research (repository state, the reference project, existing docs); *Plan* agents
  produced designs (schema changes, the CSV package, the frontend theme). Two Plan
  agents (the import pipeline and the asset-query design) failed on a session rate
  limit at 16:40, and the design was done directly from the code instead.
- **Spec-kit**: not used.
- No other AI tool appears in the recorded sessions.

## 2. Decomposition and sequencing

### 2.1 How the work was organised

1. **Foundations first, in one session (S1).** The scaffold, documentation set,
   backend layering and shared packages were established before any feature work,
   so later sessions could stay small.
2. **One concern per session, run in parallel.** Database design, logging, file
   processing and the frontend theme were four separate sessions started within 30
   minutes of each other (S2–S5), each with its own plan.
3. **Research, then decide, then build.** Sub-agents read the repository and the
   reference project; the agent then put concrete options to me as questions (with a
   recommendation); I chose; only then did it edit.
4. **Reuse over invention.** For logging, database setup, middleware, the docs
   route and generated frontend types I pointed the agent at the reference project
   and asked for a simplified port, not a copy.
5. **Every decision is written down**: `docs/ASSUMPTIONS.md` (choices),
   `docs/KNOWN_LIMITATIONS.md` (trade-offs), and this file.

### 2.2 Session timeline

| Session | Time | Workstream | Outcome |
|---|---|---|---|
| S1 `62cbfb9d` | 14:40–15:49 | Project initialisation and structure | Docker Compose stack, Go backend skeleton, Vite React + TypeScript frontend, DB migration, documentation set, layered backend, shared `base/` packages, middleware, CI skeleton |
| S2 `436755c7` | 15:39–16:03 | Database design | Lookup tables, timestamps everywhere, client IP on imports, parent-type rules in the database, all-or-nothing import; class diagram added to the architecture document |
| S3 `8f422e3d` | 15:40–16:05 | Structured logging | Trace-ID context, one-line structured format on `log/slog` |
| S4 `f5c89b18` | 15:59–16:16 | Backend file processing | `base/csvx` package with line-accurate CSV parsing and 19 tests |
| S5 `415fa031` | 16:07–16:23 | Frontend visual direction and licence | Palette tokens, system-font decision, proprietary licence file |
| S6 `dc33e20c` | 16:21–17:17 | Step 1 wrap-up and the backend API | Scaffold committed; import pipeline, endpoints, Swagger, tests |
| S7 `c16e828b` | 17:16 | Docs route | `/docs` redirects to the Swagger UI |
| S8 `c2ced89f` | 17:14–18:16 | Restructure, conventions, generated types, documentation | `service/` 19 to 8 files, gin + GORM, generated frontend types, docs sweep, this journal |
| S9 `e59ed6e6` | 18:25–20:50 | Frontend, real data, two-step import, API-key delete | Explorer UI, check → review → confirm import, overview, all-assets, activity, delete one, drawer |
| S10 `c97e4274` | 21:00–22:14 | Review refinements | One Import page, sorting, spacing scale, dark mode, status donut |
| S10 side sessions `ba46a1e3` `b96515bd` `31fabcf2` `069b64ae` `6e360f2c` | 21:49–22:05 | One-off requests | Status panel, wider import history, a git question, a CI provider swap that I reverted, centred content |
| S11 `899d268f` | 22:11 onwards | Closure check and documentation alignment | Four gaps fixed, docs and this journal aligned |
| S12 (2026-09-21) | 2026-09-21 | Local-only deployment, icons, footer, documentation refresh | CI pipeline removed, `docs/DEPLOYMENT.md`, three new type icons, footer with version and clock, docs revised |
| S13 (2026-09-22) | 2026-09-22 | Documentation cleanup and AI journal chapter | GitLab references genericised throughout the docs and PDF, new PDF chapter 15 (AI usage journal), 44 pages |

### 2.3 Detailed log

Every session in order. S1–S8 are as first written up (their times are those of that write-up);
S9 onwards were reconstructed from the transcripts.

#### S1 `62cbfb9d` — Initialisation (14:40–15:49)
- 14:40 Instruction: initialise all deliverables from the attached project documents.
  The agent produced the scaffold: Compose stack (db, migrate, backend, frontend),
  Go module with `/healthz`, Vite app, initial migration, `ARCHITECTURE.md`,
  `ACTION_PLAN.md`, `ASSUMPTIONS.md`, `KNOWN_LIMITATIONS.md`, `AI_JOURNAL.md`,
  `CLAUDE.md`.
- 14:41 I asked for React; the agent noted the architecture document specifies
  TypeScript and asked whether to drop it. **Decision: keep TypeScript.**
- 14:45 Logging: reuse the reference project's logging module, core only. The agent
  located it, found optional Kafka, Redis and gRPC sinks that would add unused
  dependencies, and asked. **Decision: core only.**
- 14:48 Added `.env` and a sample env file, `.gitignore`, and moved documentation
  under `docs/`.
- 14:49 Swagger docs with a `make docs` target and a generated API key.
- 14:53 Snake_case JSON; an assumptions document; English-only comments (the ported
  code carried non-English comments, which I had removed).
- 15:00–15:04 Clear handler, service, repository, DTO and entity separation, each
  package defined; shared helpers renamed from `util` to `base`; the ported logger
  simplified.
- 15:07 Simplified Postgres initialisation with a "sync DB" migration option.
  15:25 I questioned whether it was still needed now that the Compose `migrate`
  service applies migrations; the agent recommended removing it. **Decision: removed.**
- 15:12–15:16 A `config` package that parses every environment variable (discrete
  DB fields, no `DATABASE_URL`), a router layer, redundant `doc.go` files pruned, a
  shared JSON response helper, and a middleware layer: API key, security headers,
  CORS, rate limit, size limit.
- 15:22 Container health checks in the Dockerfiles and a CI skeleton (test and
  build on push, deploy on tag). The health checks stayed; the CI skeleton was removed on
  2026-09-21 (S12).
- 15:33 Structure clean-up, decided through the agent's questions: single `main.go`
  (no `cmd/server`), `base/postgres` as its own package, DTOs and models nested under
  `entity/`, standard-library router kept.
- 15:34–15:48 Documentation moved under `docs/` (`ACTION_PLAN` steps renamed from
  "Day" to "Step"). The agent warned that moving `CLAUDE.md` would stop Claude Code
  auto-loading it. **Decision: keep it at the root, and keep only `CLAUDE.md`
  (the duplicate `AGENTS.md` was removed).**

#### S2 `436755c7` — Database design (15:39–16:03)
- 15:39 Instructions: separate tables for asset types and operational statuses (for
  scalability), created/updated timestamps on every table, log the client IP on
  imports (there are no users), and prefer all-or-nothing imports.
- 15:45 A Plan sub-agent designed the schema change. The agent raised that strict
  all-or-nothing means the grading file, which contains a few bad rows, commits
  nothing. **Decision: keep all-or-nothing and document the trade-off.** It also
  asked whether the permitted parent types should move into a table.
  **Decision: yes.**
- 15:47 Rather than guess the exact parent-type pairs ("a wrong guess would silently
  break grading"), the agent asked; I supplied them. The agent also drew an entity-relationship
  diagram in the chat (it was not added to the documents).
- 15:59 Class diagram added to the architecture document.
- 16:02 I asked whether the validation requirements conflict. The agent showed they
  split cleanly into the two validation passes.

#### S3 `8f422e3d` — Structured logging (15:40–16:05)
- 15:40 Wanted trace-ID context in logs and a more readable structure, following the
  reference project. Explore agents located the reference and read the existing
  logger. The agent recommended replacing the hand-rolled logger with `log/slog`;
  **decision: migrate to `log/slog`.**
- 15:53 API preference: `logs.WithCtx(ctx).Warn(...)`.
- 16:00 Field-name and order changes: `ts`, `tid` (second), `lvl`, `src`, shorter
  lines. Because `slog`'s built-in handlers fix the field order, the agent wrote a
  small custom handler.

#### S4 `f5c89b18` — Backend file processing (15:59–16:16)
- 15:59 I reasoned that the backend, not the frontend, should process uploaded files,
  and asked for a minimal `base` package for it, recorded in the assumptions. Three
  Explore agents confirmed CSV processing was already backend-only in the design; a
  Plan agent designed the package. **Decision: name it `csvx`; update the architecture
  and assumptions documents.**
- Result: `Extract` and `Parse` with true source line numbers (via
  `csv.Reader.FieldPos`, so blank lines and multi-line fields cannot skew them), typed
  structural errors kept separate from per-row rejections, 19 table-driven tests.
  Committed at 16:16.

#### S5 `415fa031` — Frontend visual direction and licence (16:07–16:23)
- 16:07 Instruction: use a palette and look similar to a reference utility company's
  site; all rights reserved to me; add a copyright file. An Explore agent read the
  frontend and a Plan agent designed the theme with WCAG contrast ratios computed,
  not estimated.
- 16:10 The agent asked two questions. The reference site's typeface is proprietary
  and cannot legally be bundled, so typography is **system font stack**. Copyright
  holder: **me**.
- Result: theme tokens in `index.css`, a new favicon, a proprietary `LICENSE`, and the
  decisions recorded as assumptions. The agent verified that no brand wording remains
  anywhere in the codebase.

#### S6 `dc33e20c` — Step 1 wrap-up and the backend API (16:21–17:17)
- 16:21 "What's next? Proceed to step 2, plan accordingly." Three Explore agents read
  the plan, the backend and the frontend. The agent asked about two gaps: the grading
  CSV is referenced but not in the repository (**I said I would add it**; it was later
  committed as `backend/testdata/grid_assets.csv`), and the scaffold was uncommitted.
- 16:30 `ACTION_PLAN.md` gitignored; commit message drafted, files staged, then
  committed and pushed (16:32, `init: scaffold ...`).
- 16:35 Instruction: complete every backend endpoint to industry practice; the import
  is the priority, use Go concurrency where it helps, log rejected rows with their
  line and return them to the frontend; **columns and rows must be flexible, nothing
  hardcoded**; leave the frontend alone; wire everything into Swagger so the frontend
  can be generated from it.
- 16:38 The agent challenged the word "microservices" (it contradicts the documented
  single backend) and asked what I meant. **Decision: goroutines within one backend.**
- 16:40 Both design sub-agents failed on the session rate limit; I typed "continue"
  and the agent built the pipeline directly.
- Result: repository over real Postgres, lookup snapshot, declarative CSV schema
  (columns found by header name), pass 1 (field checks, parallel), pass 2 (hierarchy,
  cycles, cascading rejection), transactional commit, eight endpoints (the upload
  plus seven reads, including a lookups endpoint the frontend uses instead of hardcoding
  types), Swagger annotations,
  recovery and access-log middleware. Run under `-race`, then the whole stack through
  Docker with `curl`; the agent then removed the rows its manual tests had created.

#### S7 `c16e828b` — Docs route (17:16)
- 17:16 Asked for `/docs` to redirect to the Swagger UI as in the reference project;
  done, `/docs` is not key-gated but its target is.

#### S8 `c2ced89f` — Restructure, conventions, generated types, documentation (17:14–18:16)
- 17:14 Housekeeping: `service/` had grown to 19 files. Three questions (the
  difference between the two passes, whether `parallel` is generic, whether `errors.go`
  is generic) led to `base/parallel`, `base/errx` and two `csvx` helpers. Plan
  approved; 19 to 14 files.
- 17:22 A conventions list from the reference project: `TableName()` on models, a
  `Validate()` on request DTOs, model-to-DTO helpers on the model, shared constants in
  one package. Then two preferences sent mid-run: GORM for the repository and gin for
  the handlers. Both contradicted the documented stack, so the agent stopped, planned
  the change in four stages, and recorded the deviation once I approved.
- 17:37 I suggested `gomock` for repository tests; the agent declined for that layer
  (only a real database proves the queries) and offered it for handler and service
  tests. I also asked it to go faster.
- 17:46 Cleanups, a documentation sweep, a Swagger check, and generating frontend
  types from the Swagger spec the way the reference project does it. Done, with guard
  tests that fail if a route and the spec disagree.
- 17:50 Merge the import pass and schema files (and their tests) into one, and rename
  the repository import file. Done: one `service/import.go` and one `import_test.go`.
- 17:54 "What are the gaps?" The agent listed eight; most have since been closed (section 8 has what remains).
- 17:56–18:16 This journal, rewritten twice: first from the session, then from the
  transcripts.

#### S9 `e59ed6e6`, part 1 — building the frontend (18:25–20:50)

Sequence: I gave a detailed brief (a page-by-page UI concept PDF plus component,
routing, accessibility, token and testing requirements). While the agent was still
reading the repository and the API contract I sent "give me plan first"; it wrote a
plan, I approved it, and it then built in the order the brief set.

1. **Plan (read-only).** The agent read `ARCHITECTURE.md` sections 3 and 5, the
   generated OpenAPI types, the scaffold, the migrations (asset type codes) and the
   existing brand tokens, and listed the conflicts between the brief and the repo
   (below) with a default for each before any file changed.
2. **Skeleton.** Dependencies, design tokens, `AppShell` (layout only), a layout route
   with an `<Outlet/>`, `/` and `/assets/:assetId`. Type-checked before panels.
3. **Panels in the brief's order:** `ImportPanel` and `ImportResult`, `AssetTree`,
   `AssetDetailsPanel`, `SearchBar` with the select flow, then the accessibility and
   loading/empty/error passes.
4. **Verified** with the tests, the production build, the token contrast ratios, and
   a Docker Compose stack loaded with a valid and an invalid CSV over `curl` (section 5).

Decisions taken from the plan (also in `docs/ASSUMPTIONS.md`): keep React 19 rather
than the brief's 18; keep the existing scraped navy tokens rather than the concept's
green; no MSW because the backend is ready; show the import report in the main panel.

#### S9, part 2 — real data, two-step import, housekeeping

Trigger: I ran the app on the real `grid_assets.csv` and sent a screenshot showing
"222 rows, 222 rejected", plus a list of fixes (blank layout, mobile, wider asset ID
column, file chip with an X, more features, and whether a failed upload can be retried).

1. **The agent found the real file** in my Downloads folder and analysed it before
   proposing anything: 221 of 222 dates are `D/M/YY` (122 have a day above 12, none a
   month above 12, so day-first is the only reading), one date is deliberately impossible
   (`2026-13-41`), and about 16 rows are seeded traps (orphans, a cycle, wrong parent types,
   bad type/status, negative rating, missing id and name). The backend's `YYYY-MM-DD`-only
   rule, written before the file was available, was the cause of the 222/222.
2. **It surfaced the bigger consequence**: even with the date fixed, all-or-nothing would
   commit zero rows, and asked me to choose partial or all-or-nothing plus the feature
   scope. I answered partial and picked features (template, overview, view all, delete one,
   delete all, block deleting a parent), then **rejected the plan** with a better idea: make
   the user confirm before importing (check → review → confirm), and re-validate on
   confirm to avoid a race. The agent redesigned around it (preview endpoint, fingerprint,
   412/409). I rejected the plan twice more to ask how validate-first works (answered:
   stateless, re-upload rather than a staging table) and for an import activity page, which I
   then narrowed to "imports only" because deletions are not recorded there.
3. **Built backend first** (dates, `validate` shared by preview and import, list/stats/
   delete/template/activity endpoints, tests against a real Postgres, Swagger regenerated),
   then the frontend (import state machine, review screen, chip with X, overview, all
   assets, activity, delete dialogs, drawer), then the docs.
4. **Mid-run I added two backend tweaks**: trace IDs were 36-character UUIDs (now 8 hex
   characters) and the health check ran every 10 s (now every 5 minutes, with a 2 s poll
   while starting so `depends_on: service_healthy` stays fast).

#### S9, part 3 — screenshot feedback

I sent a screenshot with three complaints: deletes should need the API key, the sidebar
was cramped, and the main page looked empty. The agent could not reproduce "empty" from the
crop, so it started headless Chrome from the command line, screenshotted the running app
at 1440, 2000 and 390 px (the first time in the project it saw the UI), and found the cause:
on a wide window the content was centred in a 72 rem column, leaving a blank left area. It
also found that Chrome's minimum window width made a 390 px shot look like overflow, and
re-tested inside a 390 px iframe, which showed no overflow.

Changes: `RequireAPIKey` middleware on the DELETE route (header only, constant-time, 401,
fails closed with 403 when no key is configured); a wider rail (22/26/28 rem) with more
spacing and an "Asset tree" label; a fluid left-aligned main panel; two-column asset details
on wide screens; the bundle moved to `static/` because `/assets` is also a route.

**A misread, corrected.** The agent first read "delete should be authenticated with API key"
as "the user must enter the key" and added a key field to the delete dialogs. I corrected it:
I meant the *request* must carry the key, not that a person types it. The agent removed the
field and the browser-side storage and moved the key into the proxy: nginx (from the
container's `API_KEY`, through the image's template mechanism) and the Vite dev proxy add
`x-api-key` to DELETE requests only, replacing anything the client sent. It told me plainly
what that does and does not protect: the backend port is closed to keyless callers, but anyone
using the UI can delete. In the same message I asked for **delete-all to be reverted**, so the
endpoint, service, repository, DTO, dialog, button and tests were removed (the route now
answers 405).

While rebuilding, the agent found `frontend/nginx.conf` had been commented out in full (an
uncommitted edit that was not the agent's), which took the frontend container down. It left
the file alone and reported it (it was restored a few minutes later). Verified with `curl`
through the rebuilt stack: a DELETE through the UI's proxy with no client key returned 204,
a direct call to the backend port without a key returned 401, a wrong client key through
the proxy was replaced (204), `DELETE /api/assets` returned 405, and deleting a parent
returned 409; plus Go tests for the middleware (7 cases) and the routes.

#### S10 `c97e4274` — review refinements (21:00–22:14)

I sent a list of seven layout changes and asked to build them in a set order, stopping after
the first step to see the routing. The agent explored the frontend and the backend/docs in
two read-only sub-agents and found two things I had not known: the
"Utilities/Business/Contractor" buttons I described do not exist (the duplicated buttons were
All assets, Import activity and Download template), and the children list has no rating or
commissioned date, so sorting on those needs a small backend addition. The delete-one from
before is unchanged.

**Import page.** The agent's first plan gave Import its own page and left the activity log on
its own route. I rejected that: import and activity should be one page. The agent recommended
one page with the upload card on top, the report of the file in play under it and the history
below (no tabs, so the history stays visible and shows whether a file was already imported),
plus a jump link for long reviews. When I saw it running it still looked unorganised, so I
asked for import on the left and history on the right, to make the two halves clearer. The
seven-column history table was too wide for a half-width column, so the agent turned it into a
compact list (file, outcome, "205 of 222 rows imported · 17 rejected", date; the uploader and
the rejected rows appear when a run is opened) and gave both halves the same labelled card. On a
phone they stack, and the sideways scrolling the table needed is gone. The "Import history"
jump link now shows only when stacked. On a very wide monitor the page was still capped at
80rem, which left the history's rejection reasons cut off beside empty space, so I asked to widen
it. The agent dropped the cap (the shell already limits the width) and made the two cards 50/50
from 1536 px up; the 3:2 split stays below that. I also noticed the rejected rows in the history
were grey while the same table in the review was white: the history opens on a grey panel and the
table had no background of its own. The agent gave the table its own surface colour. Built: `/import` with no tree, an **Import** button in the
top bar on every page, `/imports` redirecting to `/import`, the sidebar reduced to the tree,
and the client state lifted above both layouts so a chosen file survives moving between pages.
Checking a file no longer navigates. It also removed the three shortcut buttons from the
Overview.

**Step 2: tree control, sorting, readable statuses.** A Collapse all icon button sits in the
tree header. The agent first built an Expand all button beside it (it loaded each level once
and then expanded everything); I asked for collapse only, so it was removed. Writing the test
for Collapse all exposed a real bug: on an asset page the tree re-opened the selected asset's
ancestors after every state change, because the tree's actions were recreated on each render, so
a collapse undid itself. The actions are now stable and a test covers it. The children list is now one table (Asset ID, Name,
Type, Status, Rating, Commissioned) with sortable headers, `aria-sort`, an arrow on the active
column and the choice kept in the URL. Rating and commissioned date were not in the children
data, so I let the agent add them to `AssetNode`; the query already loaded them, so only the DTO
and its conversion changed (plus the regenerated Swagger and TypeScript types). Sorting of that
table stays client-side, blanks sort last in both directions, and the default order is now the hierarchy
order (transformer before switchboard) rather than the database's alphabetical order. Statuses
read as "In service" everywhere; the API value is unchanged.

**Sorting, misread.** When I wrote "sorting" I meant the All assets table, not the children
list; the agent had built the latter from my first message. I sent a screenshot of the All assets
table to make it clear. That table is paged by the server, so sorting only the 25 rows on screen
would mislead; the agent added `sort` and `dir` to `GET /api/assets` instead. The column is
looked up in a fixed list of SQL expressions (never built from the request), ties break by asset
ID so pages do not shuffle, and assets with no parent sort last. Every heading in that table
(Asset ID, Name, Type, Status, Parent) sorts, the choice is kept in the URL, and a new sort goes
back to page 1. The heading component is shared with the children table. Table-driven Go tests
(ten orderings, three pages in a row, SQL text in `sort`) run against the real database. I
did not add Rating or Commissioned columns to that table; they are only in the children table.

**Layout bugs I found by looking.** After the side-by-side Import page I sent a screenshot where
the page had scrolled down and sideways at once. The agent measured every page in headless Chrome
at three window sizes (a first attempt printed nonsense because of its own measuring page) and
found that the boxes all fit the window but the document was taller than it: visually-hidden
helpers (`sr-only`, such as the file input) are absolutely positioned and, with no positioned
ancestor, escape the scrolling panel and stretch the page. I first told myself the main panel
fix was enough, because it checked out against the mock data. It was not: I then reported the
same white space again when I expanded the tree. The agent measured my real data (205 assets)
and the page was 980 px tall in a 793 px window. Every tree row has an `sr-only` status label,
and the tree's own scroll container was not positioned, so the labels of an expanded tree pushed
the page down. The tree container is now positioned, the rail clips its overflow, and the page
root (`#root`) no longer scrolls at all, since the panels scroll themselves. Measured again on
the real data at 1909x793, 1280x420 and 390x800: the page fits exactly on every route. The
lesson I took: the mock tree was too small to show it. It also gave the grid an explicit row so a tall
tree cannot grow it. The rejection table's Asset ID column was a fixed 256 px, which left a wide
gap before the reason; the row and ID columns now shrink to their content. The filter dropdowns
draw their own chevron inset from the edge, because the browser's arrow cannot be padded.
I had to point at all three; the tests passed each time. I also asked for the Download report
button to come off the rows in the import history (it stays on the review and result screens
right after an import).

**Step 3: spacing.** I asked for the audit to use one scale rather than eyeballing each
screen. The agent counted 72 off-scale utilities (30 of them `py-2.5` in table cells), mapped
each to the nearest 4/8/12/16/24/32 px step, gave the top bar, rail and main panel the same
gutters, and added a test (`src/spacing.test.ts`) that fails on any other padding, margin or
gap, so the scale cannot drift back. It checked that the test fails on a deliberate `py-2.5`.
It then ran the app against a small mock API (the dev database is empty and I did not want
sample rows in it) and screenshotted it in headless Chrome at 1440 px and, through a 390 px
iframe, on a phone width. That showed a real problem the tests could not: the children table
was squeezed into the right-hand column of the details page, clipping Rating and Commissioned
and wrapping names and status badges. The agent stacked the details card above the table so
the table has the full width. A first 390 px screenshot looked broken only because headless
Chrome will not make a window that narrow; the iframe shot was fine.

**Step 4: dark mode.** Done as a token swap: the same colour names redefined under
`data-theme="dark"`, so no component changed apart from the two dialog overlays, which had a
hard-coded black and now use an overlay token. The agent computed the contrast of every
text/background pair in both palettes before choosing the dark values (all above 4.5:1) and
recorded them. The choice is saved in `localStorage`, with the OS setting used the first time,
and a small script in `index.html` applies it before first paint. Checking it in headless
Chrome cost an extra attempt: the flag meant to force a dark colour scheme did nothing, so the
first "dark" screenshot was light. Seeding the saved choice through a temporary page made it
work, and the dark screenshots of the overview and asset details looked right.

**Verification at the end of S10.** 96 Vitest tests pass (up from 53), plus the Go tests including the
Postgres-backed router, service and repository tests (run against the dev database container,
which uses an isolated schema per test). `tsc`, `oxlint` and the production build are clean. New
tests cover the Import button on each explorer page, the missing rail on `/import`, the redirect,
keeping the chosen file across navigation, `sortChildren` (9 cases), status labels, the Collapse
all button, the list sort, and the panel positioning. This session's Go changes are covered by
tests against Postgres. Checked in headless Chrome on the pages above,
in both themes (against mock data). The Import review and result screens, the delete dialog and
the mobile drawer were not looked at in dark mode.

**Overview status panel (net result: a donut).** The "Operational status" card was removed at my
request, then brought back as a full-width row of bars after I noticed a parallel session had
removed it (the agent had not made that change and waited for my yes), then replaced at my request
by a donut beside the top-level assets. The agent loaded the data-visualisation guidance first and
chose a donut with a legend of exact counts: three parts of one whole suit a donut, and because
37 assets in maintenance and 38 out of service are close, the legend carries the numbers. Amber and
red are hard to tell apart for some colour-blind viewers, so every legend row has an icon and a
label, segments have a gap, and hovering shows the count in the centre. It is plain SVG using the
status tokens, so dark mode works. It sits beside the top-level assets from 1280 px wide and above
them on a phone.

**Contrast that looked "weird".** I sent a screenshot and said the colour contrast was a bit odd.
The agent could not view it, so it read the pixels: an Import history card with one import expanded,
made of a grey card, a white list, a grey expanded panel and a white rejection table, four alternating
near-identical tones. It flattened that to two (grey card, white content) and recorded the rule in the
assumptions. It said plainly that this was inferred from pixel analysis, not from seeing the page, and
asked me to confirm it was what I meant. I also asked for minimal mobile support to be written into the
assumptions; it is, with what was checked (a 390 px headless-Chrome pass) and what was not (real
devices, landscape, touch behaviour, 44 px tap targets).

**Side sessions.** Five short sessions ran beside this one: taking out the status panel and widening
the import history (both described above); a question on hiding older commits on GitHub (the answer:
history cannot be hidden without rewriting it, for example an orphan branch; nothing was changed in
this repository); a request to swap the CI provider, which the agent did
and I then reverted (kept the original pipeline), so the CI file stayed (until S12 removed it); and centring the page content
with `mx-auto` on the shared wrapper (not run in a browser or tested in that session).

#### S11 `899d268f` — closure check and documentation alignment (22:11 onwards)

I asked the agent to check for any gap before closing, leaving deployment out. It ran
everything rather than reading: `go vet` and `go test -race` (including the database tests,
which had been skipping because `TEST_DATABASE_URL` was not set, so it pointed them at the
dev database container; every test ran, none skipped), Vitest (96), `oxlint`, `tsc`, the
production build, and a check that the Swagger spec and `schema.ts` were in sync (it
regenerated both and compared the files; `npm run check:types` on its own is meaningless in a
tree with uncommitted changes, because it diffs against the index).

It then called the running stack and drove the UI in headless Chrome at 1440 px and 390 px, in
both themes. It found four real gaps:

- **A request that was not a valid multipart upload returned 500** (no body, a JSON body, a
  missing boundary, garbage under a boundary). `csvx.Extract` only mapped a missing file field
  to 400. It now returns 400 for those, and leaves the body-too-large case as 413; a
  table-driven test covers the four cases and I saw the three status codes on the rebuilt backend.
- **The overview's top-level cards showed "S…" instead of the asset id** at 1440 px, because
  the id and the count shared one row. The count now wraps under the id and the name may use
  two lines.
- **The CI file still had a "TODO: swap in `npm test`"**, did not run `check:types`, and ran
  the backend tests without a database so they skipped. It now runs both frontend checks
  and the backend tests against a Postgres service. I left `-race` out of CI: it needs a C
  compiler that the Alpine Go image does not have, and I cannot run the pipeline here, so this
  file was unverified, and it never ran: S12 removed it.
- **Documentation had drifted**: "no edit/delete in scope" in three places, an unfilled
  TODO checklist, "not checked in a browser" and a stale Vitest count.

**Documentation alignment.** I then asked for every Markdown file to be brought into line and for
anything unnecessary or already pruned to be removed. This journal now nests every session (S1 to
S11) under 2.3 and lists all of them in 2.2; the two placeholders in section 3 are closed; and the
stale claims are gone: that the grading CSV is not in the repository and is skipped, that the real
file has not been run, that Spec Kit is the next step, that nothing is committed, and the "gin and
GORM never run from a clean clone" gap, since the stack is rebuilt under Compose in this session.
In the other files I removed the references to the gitignored `ACTION_PLAN.md`, the "Day 2 / future"
wording for the import code that exists, the claim that the app uses the system font stack (it uses
Inter), the citations of a `CLAUDE.md` rule that is no longer there, and the duplicate
"Frontend (additions)" section of the limitations.

**Swagger opened.** Around this time I asked whether the Swagger docs should need an API key at
all. The agent checked why they did: the gate was my own S1 request, not the brief, and the endpoints
the docs describe are already open (only DELETE needs the key), so it protected nothing and made the
docs awkward for a reviewer. It removed the Swagger key middleware with its cookie and redirect logic
and their tests, kept the key on DELETE, updated the Swagger annotation, `.env.example` and the docs,
and rebuilt the backend. Checked live: `/swagger/index.html`, `doc.json` and `/docs` answer without a
key, a direct DELETE without a key is still 401, and DELETE through the UI's proxy still passes the key.

**The key on every endpoint.** Right after, I said the other endpoints should need the API key too,
and that only the Swagger docs and the health check should be open. That reverses the "no auth"
non-negotiable, so the agent recorded it as an owner-requested exception and put one
`RequireAPIKey` on the whole `/api` route group (after the rate limiter, so guessing is throttled;
fails closed with 403 when no key is set, and a start-up warning). Because the browser never holds
the key, nginx and the Vite dev proxy had to add it to every `/api` request instead of only DELETE, or
the UI would have broken. Every operation now carries `@Security ApiKeyAuth` and a test fails if one
does not. New tests call every route with no key and a wrong key (401), with the right key (reaches
the handler), on a server with no key (403), and check `/healthz` and the docs stay open. Checked live:
direct calls without the key are 401, the UI still loads all its data and previews an upload through
the proxy with no API errors. It also told me the plain consequence: anyone who can use the UI can
still use everything, because the proxy supplies the key.

Also noted: the first phone screenshot looked clipped at the right edge, but that was the
screenshot tool (headless Chrome will not go below a minimum window width); with real device
emulation there is no horizontal overflow and no console error on any page checked.

**Uneven top-level cards.** I sent a screenshot of the Overview's top-level asset cards, where the boxes
and the gaps between rows were different sizes. The cause was that each card sized to its content: a name
that wrapped to two lines made that card (and its row) taller. The agent made each card fill its grid cell
(`h-full`, a flex column), reserved two lines for the name (`min-h-[2lh]`) and pinned the ID and count line
to the bottom, so every card and every row gap is the same size. Only `HomePanel.tsx` changed. I ran the
existing `HomePanel` tests (3 pass); I did not re-check the layout in a browser.

**Scope and deployment written down.** I sent the brief's "Explicitly out of scope" list and said
deployment would be local: a CI pipeline that tests on every push, builds and deploys only on a
tag, over SSH to my own laptop, with the runner (tag `internal`) added later. The agent recorded this
in `ASSUMPTIONS.md`, noting which out-of-scope items I had since relaxed (delete one asset, the API
key, read-only lists), and pointed `KNOWN_LIMITATIONS.md` at it. I then told it to go ahead, and it
wrote the pipeline: `tags: [internal]` on every job, build and deploy on tags only, and a deploy job
that SSHes to the laptop, checks out the tag and runs `docker compose up --build -d`, using five
CI/CD variables. The YAML parses, but the pipeline has not run: there is no runner yet, and the
deploy has never reached the laptop. **Reversed on 2026-09-21 (S12):** I decided a local build and run is all
that is needed, and the whole pipeline was removed (see S12 below).

### Implementation report and user manual (2026-09-21)

I asked for a full implementation PDF with a user manual (all the screenshots needed), highlighting the
functional requirements and rendering sequence and class diagrams, and pasted the brief's section 4.1–4.3.
The agent read the docs and code, then built everything from the running system rather than from the
documentation: it started a second Compose project on other ports with its own empty database (so my dev
data was never touched), drove the real UI with headless Chrome to take 27 screenshots, imported the
supplied CSV through the UI, and drew 16 Mermaid diagrams after tracing each flow through the handler,
service and repository code. Two claims in the diagrams were wrong on first draft and fixed against the code
(the delete flow returns to the parent asset, not the overview; a `;` in a message broke Mermaid's parser).
The PDF is built from HTML with the same headless Chrome (`docs/manual/build.mjs`). The rendered pages were
looked at, which led to changes in orientation, diagram splitting and figure numbering.

It measured rather than copied the test results: `go test -race` with the real database (all 13 packages
ok, 0 skipped) and `npm test` (14 files, 96 tests). It also found a real gap while mapping the brief to the UI: `descendant_counts` was returned by the API but
never rendered, so brief 4.3's per-type counts beneath a substation were only partly met. It reported this in
the PDF as "Partial" and in `KNOWN_LIMITATIONS.md` and did not change product code, since I had asked for
documentation.

**Follow-up (same day).** I asked for the diagrams to leave out the frontend components and for the Partial to
be fixed so every requirement is met. The agent removed the frontend class diagram and redrew the sequence
diagrams and the layer diagram around a generic client and the Go layers. For the fix it added a
"Beneath this asset, at every level" row of cards to the details panel. It works out which types can sit beneath an
asset from the parent rules in `/api/lookups` (transitively, `descendantTypes` in `lib.ts`), so a substation
always shows transformers, LV boards, switchboards and switchboard panels, with 0 where there are none, and
no type name is in the code. It shares the one `/children` request with the table and shows nothing for a
panel. It first added 8 tests (3 for the helper, 5 for the panel: substation with a zero, switchboard, leaf, one
request, lookups failing, children failing), made the shared test fixtures realistic (they had empty parent
rules and empty descendant counts, which is why nothing had exercised this), and checked the result in the
real UI (Aurora: 2 transformers, 1 LV board, 2 switchboards, 10 panels).

**Design revision (same day).** I looked at the result and said two rows of cards was not informative and
asked whether the immediate-children table could carry it instead. The agent proposed three options with
previews (one merged row plus a Beneath column; expandable table rows; keep it) and I chose the first. It
replaced the two rows with one row of "Contents by type" cards (total at every level, how many are direct)
and gave the table a sortable Beneath column. Its first version of that column overflowed at 1440 px
(clipped text, names wrapping to three lines), which it saw in the screenshot and fixed by drawing the cell as
the type's icon plus a number, with the full text as a tooltip and for screen readers. 12 tests in total for
the feature; Vitest 108 pass, lint, type check and build clean. A slip: a scripted edit of the PDF's source used an empty match string and inserted the new
row between every character of one file; it was recovered by removing the inserted text and checked before
rebuilding.

### S12 — local-only deployment, icons, footer, documentation refresh (2026-09-21)

I asked, in one message, to remove the CI file fully; to record that only a local build and run is
needed, so no pipeline; to add a `DEPLOYMENT.md` with the commands and troubleshooting steps; to suggest better
icons for the LV board, switchboard and panel (substation, transformer and panel looked fine); to add a footer
modelled on a screenshot of another product's (version, copyright, current time, a link to the Swagger page);
to make every document accurate and current, with Prompt 3 of this journal brought up to the exact
implemented version; and to re-run everything for me to check before the PDF is touched.

- **Plan mode, two questions.** The agent read the code and asked two things. Icons: three sets were
  proposed (Rows3 / Columns3 / SquarePower, PlugZap / ToggleRight / DoorClosed, Network / CircuitBoard /
  LayoutPanelTop); **I chose the first**, whose three shapes read as rows of breakers, a line-up of cubicles
  and one switch unit. Footer clock: browser time zone, UTC, or the build time; **I chose the live
  browser-zone clock.**
- **Removed**: the CI pipeline file and its variables script (only used for the CI variables).
- **Footer, revised after I looked at it.** The first version was a bar pinned to the bottom of the screen. I
  said I preferred the reference screenshot's arrangement: the copyright at the bottom of the sidebar, and the
  docs and health links at the bottom right next to the timestamp, seen only by scrolling to the end of the
  page. The agent split it: `AppVersion` (name, version, copyright) at the foot of the rail and the drawer,
  and `AppFooter` (links and clock) as the last element of the scrolling main panel, using `min-h-full` so it
  sits at the bottom of a short page; Import, which has no rail, keeps the version at the left of its footer.
  It checked a short page, a tall one and Import in headless Chrome and rewrote the tests to match.
- **Footer plumbing.** The version comes from `package.json` (raised from 0.0.0 to 1.0.0) through a Vite
  `define`, so it is bumped in one place. For the Swagger link to work from the app's own port, nginx and the
  Vite proxy now forward `/swagger/` and `/docs` to the backend without adding the key. The clock is
  deliberately not an `aria-live` region. Its first version exported a helper from the component file and
  `oxlint` warned; the helper moved to `lib.ts`.
- **A change that was not the agent's.** While it worked, the Beneath column and its tests were removed from
  the working tree in a parallel change of mine; the agent noticed the diff, left it alone and reported it.
- **Tests**: new Vitest tests for the footer (version, copyright, link target, the clock ticking and its
  timer being cleared on unmount, the date format, and where the version appears with and without a rail);
  112 in total, lint, type check and build clean. Go: `go vet` and `go test -race`, with the database tests
  against the running Postgres: 119 top-level tests (167 subtests) passed, none skipped.
- **Checked in the real stack**: `docker compose up --build -d`, all services healthy; through port 8081,
  `/swagger/index.html` and `doc.json` return 200, `/docs` redirects to it, `/healthz` is up and `/api` works;
  port 8080 without a key is still 401. Headless Chrome at 1440 px showed the new icons clearly distinct and
  the footer where I wanted it. Every command written in `DEPLOYMENT.md` was run once (`ps`, `logs`, `psql`,
  `docker inspect` health, `lsof`).
- **Not fully checked**: the 390 px screenshot was cropped by headless Chrome's minimum window width, so the
  footer wrap could not be judged as a whole there; and the flag I used to try the dark theme did not switch
  it, so dark mode was not looked at (the footer uses only existing tokens, whose contrast ratios are
  recorded in `ASSUMPTIONS.md`).
- **A slip**: while updating this journal the agent wrote the file with a script that emptied it before
  reading it. It noticed at once, restored the last committed version from git and re-applied its edits; any
  uncommitted edits to this file made by anyone else in that window had to be checked by hand.
- **PDF and manual sources updated, after I had checked the app and asked for it** (v1.2, 41 pages). The agent
  started a clean stack in a second Compose project (`-p manualshots`, ports 18080 and 18081, no host port
  on the database, its own volume), extended `capture-screenshots.mjs` (the tree-revealed shot, a footer shot,
  and the loading and error states, which were done by hand before), and retook all 28 screenshots. It
  rewrote the text that had gone stale: the cover version and revision; the FR-3.4 row and section 9.5 (the Beneath
  column is gone, the cards stay); a new 9.7 on the icons and footer; chapter 10's counts (Vitest 112 in 15 files;
  Go unchanged at 119 functions, 286 with subtests); chapter 11, now "Build and deployment (local)" with the
  pipeline section replaced by everyday and troubleshooting commands; the nginx text and deployment diagram
  (Swagger is proxied); the user manual (an icon legend drawn from the same lucide sources as the app, a footer
  section and figure, the docs URL on port 8081) and chapter 14 (no CI/CD by design). The CI pipeline is still
  mentioned once, in chapter 11, as removed history. I looked at the rendered pages. Things it got wrong on the
  way, all fixed: the capture script saved the substation shot under a name the manual did not use; the first footer
  shot was identical to the details shot because that page fits on screen (retaken on the long All assets list);
  the revision on the cover used a code chip that is unreadable on the blue cover; a "v1.1" and a capital "Local"
  survived in the text. The build dependencies (mermaid, playwright-core) were installed in `docs/manual/` for the
  run and removed afterwards, as before, and the temporary stack and its volume were torn down.
- **Documentation corrected**: the README said the PDF has 43 pages and this journal said 45 (it has 43);
  `frontend/README.md` still described one layout route and an import activity page at `/imports`; the
  ARCHITECTURE infra and frontend sections gained the footer, the docs proxy and the no-pipeline note.

### S13 — documentation cleanup and AI journal chapter (2026-09-22)

I asked for every remaining "GitLab" mention in the docs to be genericised (the pipeline itself was already
removed in S12; only the wording naming the provider was left), and for this journal to be folded into the PDF
as a new chapter, condensed rather than verbatim, with a pointer back to the full file for what was left out.
The agent found 18 mentions across `docs/AI_JOURNAL.md` and `docs/ASSUMPTIONS.md`, plus two more in the PDF's
own `content-3.html`/`content-4.html` sources, and reworded each to "CI pipeline"/"CI file" while keeping the
decision history intact (a pipeline was written, never run, and removed; a later proposal to swap providers was
tried and reverted). The new chapter 15 condenses sections 1, 2.1, 2.2 and 4 of this file (tools and effort, how
the work was organised, the session timeline, and the suggestions/decisions table); the detailed log (2.3) and
representative prompts (3) were left out as too long for a report chapter. The PDF's cover revision line was
updated to the new source commit. 44 pages, still version 1.2. Rebuilt with the existing `docs/manual/build.mjs`
pipeline; no screenshots changed, so no stack was started for this session.

## 3. Representative prompts

These consolidate several short messages on the same topic from the recorded
sessions into a single, professionally worded instruction. The requirements are
unchanged; the source session and times are given. They are not transcripts; the
raw session logs are available on request.

### Prompt 1 — Project initialisation and architecture (S1, 14:40–15:34)

> Initialise all deliverables for the Smart Grid Asset Management project from the
> attached documents: a React and TypeScript (Vite) frontend, a Go backend, a
> PostgreSQL schema with migrations, Docker Compose, and the documentation set.
>
> The backend needs a clear separation of concerns: **router → handler → service →
> repository**, with `entity/model` (database structs) kept apart from `entity/dto`
> (API structs); a `config` package that parses every environment variable; a `base/`
> set of small reusable packages (logging, database connection, JSON responses); and
> a middleware layer (API key, security headers, CORS, rate and size limits). Define
> the purpose of each package.
>
> Use snake_case JSON, generate Swagger documentation through a `make docs` target,
> add `.env`, a sample env file and `.gitignore`, add health checks to the Dockerfiles,
> and add a CI skeleton. Keep only the deliverable Markdown at the repository root and
> put the rest under `docs/`. Record every assumption in `docs/ASSUMPTIONS.md`.
>
> Reuse proven patterns from my existing project for logging and database setup, but
> port only what this project needs.

- **What it produced**: the scaffold and layering described in section 2.3, plus the
  agent's clarifying questions (TypeScript, logging scope, package layout, markdown
  placement) that turned open points into recorded decisions.

### Prompt 2 — Frontend look and layout (S5 16:07, S9 18:26)

> For the frontend, take the colour palette and overall look and feel from a
> reference utility company's public website, with similar colours and, where
> applicable, similar type. The result must be original to this codebase: no company
> name, logo or lettering, and no unlicensed fonts, images or other assets, so there is
> no trademark or copyright exposure. The codebase is proprietary and all rights are
> reserved to me; add a copyright file.
>
> Build it as **one persistent app shell, not a multi-page site**: a top bar with a
> generic app name and a type-ahead search (debounced about 300 ms); a left rail with the
> import panel and the asset tree; and a main panel with the selected asset's details and
> its immediate children grouped by type, with counts. Selecting an asset never navigates
> away from the tree. Two routes share the shell through a layout route and an outlet: `/`
> (an empty-state prompt) and `/assets/:assetId` (the tree expands to and highlights that
> asset).
>
> The import panel shows total, imported and rejected counts, whether data was committed,
> and a scrollable table of rejected rows (row, asset id, reason), as its own panel and not
> a toast. The tree loads roots first and each node's children on first expand, with one
> icon per asset type, reused in the details badge, and a count on substations. Search
> results show the id, name and a type badge and work by keyboard (arrows, Enter,
> Escape); choosing one fetches the ancestors, expands them, navigates, scrolls the node
> into view and highlights it. The tree, the details and the search each have their own
> loading, empty and error states, never one global spinner.
>
> Accessibility is required: `role="tree"` and `treeitem` with `aria-expanded` and
> `aria-selected` in sync, full keyboard operation (Tab in, arrows, Right and Left to
> expand and collapse, Enter to select), visible focus rings, and body text at WCAG AA
> (4.5:1) contrast, measured. Define every colour, radius and the font once as tokens and
> use only the tokens. Test with Vitest and React Testing Library (tree, search-to-select
> call order, rejection rows from a fixture).

- **Source note**: the S5 message asked only for the palette and look, all rights reserved,
  and a copyright file; the page-by-page layout above came with the S9 brief (and a UI
  concept PDF). The no-brand-assets rule was settled in the agent's follow-up questions
  (typography: first the system font stack, later Inter, an open-licence font, in S9; copyright holder: me) and is a standing rule for the
  codebase, which the agent checked and I re-checked: no company names, logos or bundled
  third-party fonts anywhere in the repository.
- **What it produced**: five colour tokens with computed contrast ratios, a new favicon, the
  `LICENSE`, and assumptions entries (S5); the shell, panels and tests described in S9.

### Prompt 3 — Health checks and local-only build and deployment (S1 15:22, revised S11 and S12)

> Add container health checks to the Dockerfiles: poll every 2 seconds while the container is starting, so
> `depends_on: service_healthy` does not wait, and then only every 5 minutes, so the log stays quiet. Give the
> long-running services `restart: unless-stopped`.
>
> Deployment is local only: cloud deployment and Kubernetes are out of scope, so building and running with
> `docker compose up --build` on my own machine is enough. **Do not add a CI/CD pipeline** (an earlier
> CI pipeline that tested on every push and built and deployed on a tag was written, never run, and is
> removed, together with its variables script). Instead write `docs/DEPLOYMENT.md`: prerequisites, first run,
> the URLs, the everyday start, stop and rebuild commands, how to update, how to run the tests by hand, and the
> troubleshooting commands (logs, health state, port conflicts, 401 and 403 from the API key, a stale
> database volume, a look inside Postgres). Record in `docs/ASSUMPTIONS.md` that only a local build and run is
> needed and so no pipeline exists, and update every other document that mentioned the pipeline.

- **Source note**: the health checks and the original "CI skeleton" request (regression tests on every push,
  builds on every push, deploy on tag) came from S1; the tests-on-push, build-on-tag and SSH deploy design was
  written on 2026-09-20 and the pipeline was fixed in S11; the no-pipeline decision and `DEPLOYMENT.md` came
  from S12. The wording above is the final, implemented instruction.
- **What exists now**: health checks on the backend and frontend images (every 2 s while starting, then every
  5 minutes, 30 s start period), `restart: unless-stopped` on `db`, `backend` and `frontend`, and
  `docs/DEPLOYMENT.md`. Tests are run by hand (`go test -race ./...`, `npm test`, `npm run build`).
- **What was removed**: the CI pipeline file (test, build and deploy stages), its variables script,
  and the planned runner, SSH key and CI/CD variables.

## 4. Suggestions rejected, corrected or revised

| Who | What | Outcome |
|---|---|---|
| Me | Asked for React; the architecture doc said TypeScript | The agent asked; TypeScript kept |
| Agent | Port only the core of the reference project's logger (its Kafka, Redis and gRPC sinks add unused dependencies) | Accepted |
| Agent | Replace the hand-rolled logger with `log/slog` | Accepted; a small custom handler added to control field order |
| Me, then agent | I asked for a "sync DB" migration option; I later questioned it; the agent recommended removal as speculative | Removed |
| Agent | Moving all Markdown into `docs/` would stop `CLAUDE.md` auto-loading | Kept at the root; duplicate `AGENTS.md` dropped |
| Agent | The grading file would commit nothing under strict all-or-nothing | Kept all-or-nothing; trade-off documented |
| Agent | "Microservices" contradicts the single-backend design | Reinterpreted as goroutines in one backend |
| Agent | Would not guess the parent-type pairs | Asked me for the exact pairs |
| Spec, as written | Detect cycles only among rows passing earlier checks | Found by a test to make "cycle detected" unreachable; detection now runs over every in-file parent link |
| Agent | Send a large id list with `IN ?` | Would exceed Postgres's 65,535-parameter limit; single array parameter plus a 70,000-id test |
| Agent | Keep pass 1 and pass 2 in separate files | I overrode it: one `service/import.go` |
| Me | `gomock` for repository tests | Agent declined for that layer; offered for handler and service tests |
| Brief | React 18, Tailwind config file, MSW mocks | Repo already had React 19 and Tailwind 4, and the backend is ready: kept React 19 and Tailwind 4, no MSW; recorded as assumptions |
| Concept PDF | Import result shows "214 imported / 8 rejected" as committed | The API is all-or-nothing, so the UI leads with committed yes/no and shows imported as 0 when nothing was stored |
| Brief | Stop and show the skeleton before building the panels | The same message asked for a complete draft, so I treated approving the plan as the checkpoint; the agent said so in the plan |
| Me, then agent | Agent asked "partial or all-or-nothing?" for the real file, which has ~17 bad rows | I replaced the question with check → review → confirm (validate first, then confirm, re-validating to avoid races); the agent designed the fingerprint/412/409 flow |
| Agent | Considered staging the uploaded file server-side between preview and confirm | Rejected by the agent itself: the browser keeps the file and re-uploads it, so there is no staging table, expiry or cleanup, and the backend never trusts a client-sent outcome |
| Me | Import activity page | Agreed, then narrowed: label it "imports only" (deletions are not in the import tables), no new migration |
| Agent | Tree rows use `aria-owns` for their children | Tests exposed that an expanded row's accessible name then included its whole subtree; fixed with `aria-labelledby` on each row |
| Me | Model-to-DTO helpers on the model | Agent flagged that `model` then imports `dto`; built as asked and documented |
| Agent, then me | Standard-library router recommended and kept at 15:33 | Later replaced by gin at my request, with the deviation recorded |
| Me | Expand all button beside Collapse all in the tree | Built, then removed at my request: collapse only |
| Me, then agent | "Sorting" meant the All assets table, not the children list | The agent added server-side `sort` and `dir`, because sorting only the 25 rows on screen would mislead |
| Me | Swap the CI provider | The agent did it; I reverted: kept the original pipeline |
| Me (S12) | Remove the CI pipeline altogether; a local build and run is enough | Done, with the assumption recorded and `DEPLOYMENT.md` written; the earlier decision to keep it was superseded |
| Agent, then me (S12) | Three icon sets proposed for the LV board, switchboard and panel | I chose Rows3 / Columns3 / SquarePower |
| Agent, then me (S12) | Footer clock: browser time zone, UTC or build time | I chose the live browser-zone clock |
| Agent, then me (S12) | Footer as a bar fixed to the bottom of the screen | I preferred the reference's layout: copyright at the foot of the sidebar, links and clock at the end of the page; changed |
| Me (S1), then me (S11) | Swagger docs behind the API key | Removed in S11: with the endpoints open at the time, the gate protected nothing |
| Me | Only DELETE needs the key | Changed in S11: every `/api` endpoint needs it; only `/healthz` and the docs are open |

## 5. How generated code was reviewed and verified

- **Decisions before code.** 14 rounds of clarifying questions (24 questions in all) and
  the plan approvals meant I chose the direction before files changed, and I sent about
  30 mid-run redirections while it worked.
- **Tests as the gate.** 57 `go test` runs (26 with `-race`), 80 build or vet runs,
  against a real Postgres with an isolated schema per test, so dev data is never
  touched.
- **End to end.** 42 Docker Compose commands and 20 `curl` calls: the first backend
  build was run through the whole stack, then the test data was removed.
- **"Green means it ran."** A full run once reported `ok` with 21 database tests
  silently skipped (`TEST_DATABASE_URL` unset); the agent noticed, started Postgres
  and re-ran. Nothing skips now: the fixture CSV is committed.
- **Behavioural parity.** After the gin and GORM change, the same 37 requests were
  replayed against the old and new servers: identical statuses and bodies except three
  intended differences (JSON bodies for 404 and 405, `charset=utf-8`, no implicit
  `HEAD`).
- **A regression test for each defect found**: cycle reporting, cascade-reason
  ordering, log forging through filenames and cells, the blank Swagger page, and the
  parameter limit.
- **Contract checks.** Swagger regenerated byte-identical; tests fail if a route and
  the spec disagree, a `$ref` dangles, or the UI is reachable without the API key;
  the generated frontend types are deterministic and type-check.
- **Frontend (S9).** 24 Vitest tests (tree expand/collapse and lazy fetch, selection and
  `aria-selected`, keyboard movement and a single tab stop, search → ancestors →
  expand → navigate call order, rejection rows from a fixture, per-panel error and
  empty states, the expansion reducer); `tsc -b`, `oxlint` and `vite build` clean. A
  script measured WCAG contrast for every text/background token pair (lowest 5.31:1).
  A real Docker Compose stack was loaded with a valid CSV (7 rows committed) and an
  invalid one (3 rejections, nothing committed), and roots, search and ancestors were
  read back through nginx, matching the fixtures' shapes. That sample data is still in
  the dev database. **No browser was driven**: layout, focus rings and the drag-and-drop
  path had not been looked at on screen at that point (later rounds did).
- **Second round (S9).** The real file was run through the whole stack twice: first as
  a Go test through `router.New` on an isolated schema (222 rows, 205 importable, 17
  rejected, each seeded trap checked for its own reason, no date rejection except the
  impossible one), then against a freshly rebuilt Docker Compose stack over `curl`:
  preview stored nothing, a wrong fingerprint returned 412, the confirmed import stored
  205 rows with dates such as `12/3/10` saved as 2010-03-12, re-uploading the file stored
  nothing (every row a duplicate), deleting a parent returned 409, delete-all (since removed) needed the
  confirm value, import history survived it, and the template imported cleanly. 117 Go
  tests (all ran, with a real Postgres) and 55 Vitest tests; `tsc`, `oxlint` and
  `vite build` clean; swagger regenerated and the router test that compares routes to the
  spec passes.
- **S10 and S11.** 96 Vitest tests, about 120 Go tests all run against a real Postgres (none
  skipped), `oxlint`, `tsc` and the build clean; the Swagger spec and `schema.ts` regenerated and
  compared; the running stack called with `curl` and the pages driven in headless Chrome at
  1440 px and 390 px in both themes, with no overflow and no console errors. The CI file
  never ran and was removed in S12.
- **S12.** 112 Vitest tests, 119 top-level Go tests (167 subtests) with a real Postgres, none skipped,
  `go vet`, `oxlint`, the types check and the build clean; the stack rebuilt and probed with `curl`, and the
  pages captured in headless Chrome at 1440 px (light theme only; see S12 for what was not checked).
- **Not claimed**: this journal makes no statement about reading every diff line by
  line; review was decision-level and evidence-level.

## 6. Where the agent did poorly or needed more context

- **Ported code carried non-English comments**, which I had to ask to be removed.
- **Speculative or duplicated artefacts**: a "sync DB" option nobody needed, and two
  agent-instruction files with the same content (`AGENTS.md` and `CLAUDE.md`) that I had
  merged into one.
- **Documentation lagged behind code.** After the refactors, the docs still described
  the standard-library router and `database/sql` until the documentation sweep.
- **It needed context it could not find**: the grading CSV (added later),
  the exact parent-type pairs, where the reference project lives, and what I meant by
  "microservices". It asked instead of guessing, which was the right behaviour.
- **Rate limits** killed two design sub-agents.
- **Defects in the first backend build**, each found by testing: an unreachable cycle
  reason, a cascade reason that depended on row order, log lines forgeable through
  filenames, and a Swagger UI that rendered blank under the strict
  Content-Security-Policy.
- **Third round (S9).** It took "needs the API key" to mean a key field for the user, built it
  with tests and documentation, and only then heard that the request should carry the key.
  The requirement was ambiguous and the agent did not ask.
- **Second round (S9).** My earlier "all-or-nothing" decision had been made without the real
  file, and the agent had documented the consequence but not tested it; the first run of
  the real data exposed both the date format and the seeded bad rows at once. Its own
  first tests were wrong twice (a sort order and a status count) before the code was.
  It again could not look at the screen, so the mobile drawer, table scrolling and focus
  rings are tested in jsdom only.
- **Frontend session (S9).** The agent's first tree gave every expanded row a
  screen-reader name containing its whole subtree (`aria-owns` plus name-from-content);
  only the tests caught it. Its first test client used TanStack Query's default
  `staleTime`, so a "served from cache" assertion failed until the client matched the
  app's. It also could not verify anything visually, which is why that gap is listed.
- **Layout and closure (S10, S11).** It read "sorting" as the wrong table, tested the layout against
  a mock tree too small to show a page-height bug until I reported it twice, and its first "dark
  mode" screenshot was light because the forcing flag did nothing. In S11 it found that the
  database tests had been skipping, and the 500 on a malformed upload, by running things instead
  of reading them.
- **Tooling slips in S9**: the edit tool turned a Unicode escape into
  a literal invisible character (twice, a compile error); a scripted merge mistook the
  module path for standard library and duplicated the import block; a regeneration
  command with its output discarded failed silently and produced a false "files
  differ"; and the green-with-skips near-miss above.

- **Report and manual (2026-09-21).** The first screenshot run was invalid: the agent changed
  `BACKEND_PORT` for the isolated stack, but that variable is also the port the backend listens on inside the
  container, so nginx returned 502 for every API call and the "empty" and "error" pages it captured were
  wrong. It noticed from the timeouts and the nginx log, rebuilt the stack with only the host port remapped,
  and retook everything. It also copied 215 MB of `node_modules` into the repository to run the PDF build
  (removed afterwards, and now ignored), and the first PDF layout had unreadable diagrams that only showed up
  when the rendered pages were looked at.

## 7. Approximate human working time

About **3 hours hands-on**, from the timestamps of my messages and answers: between
roughly 2¼ and 3¼ hours depending on how long an idle gap still counts as engaged,
inside a 3.6-hour window on 2026-09-20 (14:40–18:16). Three long unattended stretches
(about 56 minutes in total, mostly the backend build) are excluded. Eight sessions add
up to about 4.5 hours of session time because up to four ran at once. Sessions S9 onwards are not counted here. Time spent
outside Claude Code (reading the brief, browsing the reference sites) is not captured.

## 8. Known gaps and next steps

Full list in [`KNOWN_LIMITATIONS.md`](KNOWN_LIMITATIONS.md). The gaps that matter:

- **The whole `/api` is guarded by one shared API key** (no user accounts) that nginx adds
  itself, so the UI's users can still do everything, including delete; deletions are only in the
  server log, not on the Import activity page.
- **The frontend has only had a light browser check.** S10 and the closure check (S11)
  looked at the main pages in headless Chrome (both themes, desktop and 390 px, against the real
  data), but not at focus rings or drag-and-drop. The delete dialog, import review and report screens, loading and
  error states and the mobile drawer were captured on 2026-09-21 for the manual. There is no end-to-end test and no screen-reader check.
- **A pending import review is lost on refresh** (the chosen file is held in memory); past
  imports are on the Import activity page.
- **No CI/CD, by design.** Only a local build and run is needed, so there is no pipeline and no remote
  target; the tests are run by hand (`docs/DEPLOYMENT.md`).
- **Bulk-insert speed is unmeasured** since the move to GORM.
- What I would do next is in [`KNOWN_LIMITATIONS.md`](KNOWN_LIMITATIONS.md).

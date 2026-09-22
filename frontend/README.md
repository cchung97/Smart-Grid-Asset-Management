# Frontend

React + TypeScript (Vite), Tailwind, served by nginx in Docker Compose, which
also proxies `/api/*` (adding the API key), `/healthz` and the open Swagger docs (`/swagger/`, `/docs`) to the backend. The frontend only ever
talks to the Go API. See `../docs/ARCHITECTURE.md` section 5 for the intended
structure (import panel, asset tree, details panel, search).

## Structure

Single-page app: `AppRoot` holds the client state and wraps two layout routes that
share `AppShell` (top bar, main panel, `AppFooter`). `ExplorerLayout` adds the tree rail for `/`
overview, `/assets` all assets and `/assets/:assetId` one asset; `PageLayout` has no rail and
serves `/import` (upload, review and history; `/imports` redirects there). Below `md` the rail is a
drawer (Radix Dialog). `AppVersion` (version from `package.json`, injected as `__APP_VERSION__` by `vite.config.ts`, and the
copyright) sits at the bottom of the rail and drawer; `AppFooter` (links to the API docs and health check, and a
live clock) is the last element of the scrolling main panel, not a fixed bar. TanStack Query owns all
server state; the only client state is the set of expanded tree nodes, the
two-step import flow (`state/useImportFlow.ts`: idle → selected → checking →
preview → importing → done, holding the chosen `File` in memory) and the drawer.
Design tokens (colours, radii, Inter font) live only in `src/index.css`. Tests sit
beside the components (`*.test.tsx`); `src/test/utils.tsx` stubs `fetch` with a
route table and records call order.

## Scripts

```bash
npm run dev             # Vite dev server
npm run build           # type-check (tsc -b) and build
npm test                # Vitest + React Testing Library (npm run test:watch to watch)
npm run lint            # oxlint
npm run generate:types  # regenerate src/api/schema.ts from ../backend/docs/swagger.json
npm run check:types     # fails if src/api/schema.ts is stale
```

## API types are generated

`src/api/schema.ts` is generated from the backend's OpenAPI spec — do not edit it
and do not hand-write a type the backend already describes:

1. the backend regenerates its spec (`cd ../backend && make docs`);
2. `npm run generate:types` converts Swagger 2.0 to OpenAPI 3 (`swagger2openapi`,
   into the git-ignored `src/api/openapi3.json`) and generates the types
   (`openapi-typescript`);
3. commit `src/api/schema.ts`.

`src/api/types.ts` gives the schemas readable names:

```ts
import type { AssetNode, ImportResponse, SearchQuery } from "./api/types";
```

Every response field is required and nullable ones are `T | null`, matching what
the API returns. `openapi-typescript` declares a TypeScript 5 peer; `package.json`
has an `overrides` entry so it uses this project's TypeScript 6.

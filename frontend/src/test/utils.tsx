import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render } from '@testing-library/react'
import { vi } from 'vitest'
import { MemoryRouter, useLocation } from 'react-router-dom'
import App from '../App'
import type {
  AncestorsResponse,
  AssetDetail,
  AssetListResponse,
  AssetNode,
  ChildrenResponse,
  ImportListResponse,
  ImportPreviewResponse,
  LookupsResponse,
  RootsResponse,
  SearchResponse,
  StatsResponse,
} from '../api/types'

// ---- fixtures: SUB-1 ─┬─ TX-1
//                       └─ SWB-1 ── PNL-1
const summary = ({ child_count: _c, subtree_count: _s, rating_kva: _r, commissioned_date: _d, ...rest }: AssetNode) => rest

const node = (
  asset_id: string,
  asset_type: string,
  parent: string | null,
  child_count: number,
  subtree_count: number,
  rating_kva: number | null = null,
  commissioned_date: string | null = null,
): AssetNode => ({
  asset_id,
  asset_name: `${asset_id} name`,
  asset_type,
  parent_asset_id: parent,
  operational_status: 'IN_SERVICE',
  rating_kva,
  commissioned_date,
  child_count,
  subtree_count,
})

export const SUB = node('SUB-1', 'SUBSTATION', null, 2, 4)
export const TX = node('TX-1', 'TRANSFORMER', 'SUB-1', 0, 1, 1000, '2019-03-14')
export const SWB = node('SWB-1', 'SWITCHBOARD', 'SUB-1', 1, 2, 500, null)
export const PNL = node('PNL-1', 'SWITCHBOARD_PANEL', 'SWB-1', 0, 1)

const group = (asset_type: string, assets: AssetNode[]) => ({
  asset_type,
  count: assets.length,
  subtree_count: assets.reduce((n, a) => n + a.subtree_count, 0),
  assets,
})

export const detail = (n: AssetNode): AssetDetail => ({
  ...summary(n),
  voltage_kv: 22,
  rating_kva: null,
  manufacturer: 'Meridian',
  model: 'M-1',
  serial_number: 'SN-1',
  commissioned_date: '2019-03-14',
  created_at: '2026-01-01T00:00:00Z',
  updated_at: '2026-01-01T00:00:00Z',
})

export const rootsBody = (roots: AssetNode[] = [SUB]): RootsResponse => ({
  total: roots.length,
  roots,
})

export const statsBody = (total = 4): StatsResponse => ({
  total,
  by_type: total
    ? [
        { asset_type: 'SUBSTATION', count: 1 },
        { asset_type: 'SWITCHBOARD', count: 1 },
        { asset_type: 'SWITCHBOARD_PANEL', count: 1 },
        { asset_type: 'TRANSFORMER', count: 1 },
      ]
    : [],
  by_status: total ? [{ operational_status: 'IN_SERVICE', count: total }] : [],
})

export const lookupsBody: LookupsResponse = {
  asset_types: ['LV_BOARD', 'SUBSTATION', 'SWITCHBOARD', 'SWITCHBOARD_PANEL', 'TRANSFORMER'],
  operational_statuses: ['IN_SERVICE', 'MAINTENANCE', 'OUT_OF_SERVICE'],
  parent_rules: [
    { child_type: 'TRANSFORMER', parent_type: 'SUBSTATION' },
    { child_type: 'LV_BOARD', parent_type: 'SUBSTATION' },
    { child_type: 'SWITCHBOARD', parent_type: 'SUBSTATION' },
    { child_type: 'SWITCHBOARD_PANEL', parent_type: 'SWITCHBOARD' },
  ],
  root_types: ['SUBSTATION'],
}

export const listBody = (items = [SUB, SWB, TX, PNL], total = items.length): AssetListResponse => ({
  total,
  limit: 25,
  offset: 0,
  items: items.map(summary),
})

export const previewBody: ImportPreviewResponse = {
  total_rows: 222,
  importable_rows: 205,
  rejected_rows: 2,
  ignored_columns: [],
  fingerprint: 'fp-1',
  rejections: [
    { csv_row: 178, asset_id: 'TX-NR', reason: 'rating_kva cannot be negative (-500)' },
    { csv_row: 183, asset_id: 'PNL-001-1-01', reason: 'cycle detected: PNL-001-1-01 -> PNL-001-1-02 -> PNL-001-1-01' },
  ],
}

export const importsBody: ImportListResponse = {
  total: 1,
  limit: 25,
  offset: 0,
  items: [
    {
      import_id: 'run-1',
      filename: 'grid_assets.csv',
      client_ip: '203.0.113.7',
      total_rows: 222,
      imported_rows: 205,
      rejected_rows: 17,
      committed: true,
      created_at: '2026-09-20T08:15:00Z',
    },
  ],
}

export const defaultRoutes: Record<string, unknown> = {
  'GET /api/assets/stats': statsBody(),
  'GET /api/lookups': lookupsBody,
  'GET /api/assets?limit=25&offset=0': listBody(),
  'GET /api/imports?limit=25&offset=0': importsBody,
  'GET /api/imports/run-1': { ...importsBody.items[0], import_id: 'run-1', committed: true, ignored_columns: [], rejections: previewBody.rejections },
  'GET /api/assets/roots': rootsBody(),
  'GET /api/assets/SUB-1': detail(SUB),
  'GET /api/assets/TX-1': detail(TX),
  'GET /api/assets/PNL-1': detail(PNL),
  'GET /api/assets/SUB-1/children': {
    asset_id: 'SUB-1',
    total_children: 2,
    // beneath SUB-1: TX-1 and SWB-1 directly, PNL-1 under SWB-1; no LV board
    descendant_counts: [
      { asset_type: 'SWITCHBOARD', count: 1 },
      { asset_type: 'SWITCHBOARD_PANEL', count: 1 },
      { asset_type: 'TRANSFORMER', count: 1 },
    ],
    groups: [group('SWITCHBOARD', [SWB]), group('TRANSFORMER', [TX])],
  } satisfies ChildrenResponse,
  'GET /api/assets/SWB-1/children': {
    asset_id: 'SWB-1',
    total_children: 1,
    descendant_counts: [{ asset_type: 'SWITCHBOARD_PANEL', count: 1 }],
    groups: [group('SWITCHBOARD_PANEL', [PNL])],
  } satisfies ChildrenResponse,
  'GET /api/assets/TX-1/children': { asset_id: 'TX-1', total_children: 0, groups: [], descendant_counts: [] },
  'GET /api/assets/PNL-1/children': { asset_id: 'PNL-1', total_children: 0, groups: [], descendant_counts: [] },
  'GET /api/assets/TX-1/ancestors': { asset_id: 'TX-1', path: [summary(SUB), summary(TX)] } satisfies AncestorsResponse,
  'GET /api/assets/PNL-1/ancestors': {
    asset_id: 'PNL-1',
    path: [summary(SUB), summary(SWB), summary(PNL)],
  } satisfies AncestorsResponse,
  'GET /api/assets/search?q=panel&limit=20': {
    query: 'panel',
    type: '',
    count: 1,
    truncated: false,
    results: [summary(PNL)],
  } satisfies SearchResponse,
}

type Handler = unknown | ((init?: RequestInit) => unknown)

/**
 * Replaces global fetch with a route table keyed "METHOD /path?query" and
 * returns the recorded calls (in order) so tests can assert call sequences.
 * A route whose value is a `Response` is returned as-is (for error cases).
 */
export function mockApi(overrides: Record<string, Handler> = {}) {
  const routes: Record<string, Handler> = { ...defaultRoutes, ...overrides }
  const calls: string[] = []
  vi.stubGlobal(
    'fetch',
    vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
      const key = `${init?.method ?? 'GET'} ${String(input)}`
      calls.push(key)
      if (!(key in routes)) {
        return Response.json({ error: `unmocked ${key}` }, { status: 404 })
      }
      const h = routes[key]
      const out = typeof h === 'function' ? (h as (i?: RequestInit) => unknown)(init) : h
      return out instanceof Response ? out.clone() : Response.json(out)
    }),
  )
  return calls
}

// oxlint-disable-next-line react/only-export-components -- test helper file, no fast refresh
function LocationProbe() {
  return <div data-testid="location">{useLocation().pathname}</div>
}

export function renderApp(path = '/') {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false, staleTime: 30_000 } } })
  return render(
    <QueryClientProvider client={client}>
      <MemoryRouter initialEntries={[path]}>
        <App />
        <LocationProbe />
      </MemoryRouter>
    </QueryClientProvider>,
  )
}

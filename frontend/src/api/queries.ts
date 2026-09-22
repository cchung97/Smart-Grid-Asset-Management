import { keepPreviousData, QueryClient, useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { ApiError, apiFetch } from './client'
import type {
  AncestorsResponse,
  AssetDetail,
  AssetListResponse,
  ChildrenResponse,
  ImportListResponse,
  ImportPreviewResponse,
  ImportResponse,
  LookupsResponse,
  RootsResponse,
  SearchResponse,
  StatsResponse,
} from './types'

export const SEARCH_LIMIT = 20
export const PAGE_SIZE = 25

export interface AssetListParams {
  q: string
  type: string
  status: string
  sort?: string // an AssetListQuery['sort'] key; absent means asset_id
  dir?: 'asc' | 'desc'
  page: number // 1-based
}

/** All server-state keys live here so invalidation and prefetching agree. */
export const keys = {
  all: ['assets'] as const,
  roots: ['assets', 'roots'] as const,
  detail: (id: string) => ['assets', id, 'detail'] as const,
  children: (id: string) => ['assets', id, 'children'] as const,
  ancestors: (id: string) => ['assets', id, 'ancestors'] as const,
  search: (q: string) => ['search', q] as const,
  list: (p: AssetListParams) => ['assets', 'list', p] as const,
  stats: ['assets', 'stats'] as const,
  lookups: ['lookups'] as const,
  imports: (page: number) => ['imports', 'list', page] as const,
  importDetail: (id: string) => ['imports', 'detail', id] as const,
  allImports: ['imports'] as const,
}

const enc = encodeURIComponent

export const api = {
  roots: () => apiFetch<RootsResponse>('/api/assets/roots'),
  asset: (id: string) => apiFetch<AssetDetail>(`/api/assets/${enc(id)}`),
  children: (id: string) => apiFetch<ChildrenResponse>(`/api/assets/${enc(id)}/children`),
  ancestors: (id: string) => apiFetch<AncestorsResponse>(`/api/assets/${enc(id)}/ancestors`),
  search: (q: string) =>
    apiFetch<SearchResponse>(`/api/assets/search?q=${enc(q)}&limit=${SEARCH_LIMIT}`),
  lookups: () => apiFetch<LookupsResponse>('/api/lookups'),
  stats: () => apiFetch<StatsResponse>('/api/assets/stats'),
  assetList: (p: AssetListParams) => {
    const qs = new URLSearchParams({ limit: String(PAGE_SIZE), offset: String((p.page - 1) * PAGE_SIZE) })
    if (p.q) qs.set('q', p.q)
    if (p.type) qs.set('type', p.type)
    if (p.status) qs.set('status', p.status)
    if (p.sort && p.sort !== 'asset_id') qs.set('sort', p.sort)
    if (p.dir === 'desc') qs.set('dir', 'desc')
    return apiFetch<AssetListResponse>(`/api/assets?${qs}`)
  },
  // The API key the backend requires for DELETE is added by the proxy (nginx / Vite), not here.
  deleteAsset: (id: string) => apiFetch<void>(`/api/assets/${enc(id)}`, { method: 'DELETE' }),
  importList: (page: number) =>
    apiFetch<ImportListResponse>(`/api/imports?limit=${PAGE_SIZE}&offset=${(page - 1) * PAGE_SIZE}`),
  importDetail: (id: string) => apiFetch<ImportResponse>(`/api/imports/${enc(id)}`),
  /** Validates the file and stores nothing. */
  previewImport: (file: File) => {
    const body = new FormData()
    body.append('file', file)
    return apiFetch<ImportPreviewResponse>('/api/imports/preview', { method: 'POST', body })
  },
  /** Validates again and stores the valid rows; refused (412) if `fingerprint` no longer matches. */
  commitImport: (file: File, fingerprint: string) => {
    const body = new FormData()
    body.append('file', file)
    body.append('expected_fingerprint', fingerprint)
    return apiFetch<ImportResponse>('/api/imports', { method: 'POST', body })
  },
}

/** Client errors (4xx) will not fix themselves; only retry transient failures once. */
export function createQueryClient(): QueryClient {
  return new QueryClient({
    defaultOptions: {
      queries: {
        staleTime: 30_000,
        refetchOnWindowFocus: false,
        retry: (count, error) =>
          !(error instanceof ApiError && error.status >= 400 && error.status < 500) && count < 1,
      },
    },
  })
}

export const useRoots = () => useQuery({ queryKey: keys.roots, queryFn: api.roots })

export const useAsset = (id: string) =>
  useQuery({ queryKey: keys.detail(id), queryFn: () => api.asset(id) })

export const useChildren = (id: string, enabled = true) =>
  useQuery({ queryKey: keys.children(id), queryFn: () => api.children(id), enabled })

export const useAncestors = (id: string | undefined) =>
  useQuery({
    queryKey: keys.ancestors(id ?? ''),
    queryFn: () => api.ancestors(id as string),
    enabled: !!id,
  })

export const useSearch = (q: string) =>
  useQuery({
    queryKey: keys.search(q),
    queryFn: () => api.search(q),
    enabled: q.length > 0,
    staleTime: 0,
  })

/** Imperative fetch used by the search → select flow (cached like any query). */
export const fetchAncestors = (client: QueryClient, id: string) =>
  client.fetchQuery({ queryKey: keys.ancestors(id), queryFn: () => api.ancestors(id) })

export const useStats = () => useQuery({ queryKey: keys.stats, queryFn: api.stats })

export const useLookups = () => useQuery({ queryKey: keys.lookups, queryFn: api.lookups, staleTime: 5 * 60_000 })

export const useAssetList = (params: AssetListParams) =>
  useQuery({
    queryKey: keys.list(params),
    queryFn: () => api.assetList(params),
    placeholderData: keepPreviousData, // keep the old page on screen while the next one loads
  })

export const useImportList = (page: number) =>
  useQuery({
    queryKey: keys.imports(page),
    queryFn: () => api.importList(page),
    placeholderData: keepPreviousData,
    staleTime: 0,
  })

export const useImportDetail = (id: string, enabled: boolean) =>
  useQuery({ queryKey: keys.importDetail(id), queryFn: () => api.importDetail(id), enabled })

/** Refresh everything that shows stored assets or import history. */
export function useInvalidateAssets() {
  const client = useQueryClient()
  return () => {
    void client.invalidateQueries({ queryKey: keys.all })
    void client.invalidateQueries({ queryKey: keys.allImports })
    void client.invalidateQueries({ queryKey: ['search'] })
  }
}

export function useDeleteAsset() {
  const invalidate = useInvalidateAssets()
  return useMutation({ mutationFn: api.deleteAsset, onSuccess: invalidate })
}

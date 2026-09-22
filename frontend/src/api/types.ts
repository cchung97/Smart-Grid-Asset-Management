// Friendly names for the API types. Everything here is derived from
// ./schema.ts, which `npm run generate:types` regenerates from the backend's
// OpenAPI spec (backend/docs/swagger.json) — so a backend change that breaks
// a use of these types fails `npm run build` instead of failing at runtime.
// Do not hand-write a type for something the backend already describes.
import type { components, paths } from "./schema";

type Schemas = components["schemas"];

export type AssetSummary = Schemas["dto.AssetSummary"];
export type AssetDetail = Schemas["dto.AssetDetail"];
export type AssetNode = Schemas["dto.AssetNode"];
export type ChildGroup = Schemas["dto.ChildGroup"];
export type TypeCount = Schemas["dto.TypeCount"];
export type RootsResponse = Schemas["dto.RootsResponse"];
export type ChildrenResponse = Schemas["dto.ChildrenResponse"];
export type AncestorsResponse = Schemas["dto.AncestorsResponse"];
export type SearchResponse = Schemas["dto.SearchResponse"];
export type LookupsResponse = Schemas["dto.LookupsResponse"];
export type ParentRule = Schemas["dto.ParentRule"];
export type ImportResponse = Schemas["dto.ImportResponse"];
export type ImportPreviewResponse = Schemas["dto.ImportPreviewResponse"];
export type ImportSummary = Schemas["dto.ImportSummary"];
export type ImportListResponse = Schemas["dto.ImportListResponse"];
export type AssetListResponse = Schemas["dto.AssetListResponse"];
export type StatsResponse = Schemas["dto.StatsResponse"];
export type StatusCount = Schemas["dto.StatusCount"];
export type Rejection = Schemas["dto.Rejection"];
export type HealthResponse = Schemas["dto.HealthResponse"];
/** Body of every non-2xx response that has no richer shape of its own. */
export type ErrorResponse = Schemas["dto.ErrorResponse"];

/** Query parameters of GET /api/assets/search. */
export type SearchQuery = NonNullable<paths["/api/assets/search"]["get"]["parameters"]["query"]>;

/** Query parameters of GET /api/assets. */
export type AssetListQuery = NonNullable<paths["/api/assets"]["get"]["parameters"]["query"]>;

package dto

// AssetSummary is the compact form of an asset used in lists, paths and
// search results.
type AssetSummary struct {
	AssetID           string  `json:"asset_id" example:"TX-001-1" validate:"required"`
	ParentAssetID     *string `json:"parent_asset_id" example:"SUB-001" validate:"required" extensions:"x-nullable"`
	AssetType         string  `json:"asset_type" example:"TRANSFORMER" validate:"required"`
	AssetName         string  `json:"asset_name" example:"Transformer 1" validate:"required"`
	OperationalStatus string  `json:"operational_status" example:"IN_SERVICE" validate:"required"`
}

// AssetDetail is every stored attribute of one asset.
type AssetDetail struct {
	AssetID           string   `json:"asset_id" example:"TX-001-1" validate:"required"`
	ParentAssetID     *string  `json:"parent_asset_id" example:"SUB-001" validate:"required" extensions:"x-nullable"`
	AssetType         string   `json:"asset_type" example:"TRANSFORMER" validate:"required"`
	AssetName         string   `json:"asset_name" example:"Transformer 1" validate:"required"`
	OperationalStatus string   `json:"operational_status" example:"IN_SERVICE" validate:"required"`
	VoltageKV         *float64 `json:"voltage_kv" example:"22" validate:"required" extensions:"x-nullable"`
	RatingKVA         *float64 `json:"rating_kva" example:"1000" validate:"required" extensions:"x-nullable"`
	Manufacturer      *string  `json:"manufacturer" example:"ABB" validate:"required" extensions:"x-nullable"`
	Model             *string  `json:"model" example:"RESIBLOC" validate:"required" extensions:"x-nullable"`
	SerialNumber      *string  `json:"serial_number" example:"SN-99812" validate:"required" extensions:"x-nullable"`
	// CommissionedDate is formatted YYYY-MM-DD.
	CommissionedDate *string `json:"commissioned_date" example:"2019-04-30" validate:"required" extensions:"x-nullable"`
	CreatedAt        string  `json:"created_at" format:"date-time" example:"2026-09-20T08:15:00Z" validate:"required"`
	UpdatedAt        string  `json:"updated_at" format:"date-time" example:"2026-09-20T08:15:00Z" validate:"required"`
}

// AssetNode is an asset plus what the explorer tree needs to render it
// without a further request: whether it can be expanded and how big its
// subtree is.
type AssetNode struct {
	AssetID           string  `json:"asset_id" example:"SWB-001-1" validate:"required"`
	ParentAssetID     *string `json:"parent_asset_id" example:"SUB-001" validate:"required" extensions:"x-nullable"`
	AssetType         string  `json:"asset_type" example:"SWITCHBOARD" validate:"required"`
	AssetName         string  `json:"asset_name" example:"Switchboard 1" validate:"required"`
	OperationalStatus string  `json:"operational_status" example:"IN_SERVICE" validate:"required"`
	// RatingKVA and CommissionedDate let a list of nodes (the children of an
	// asset) be sorted without a request per row. CommissionedDate is YYYY-MM-DD.
	RatingKVA        *float64 `json:"rating_kva" example:"1000" validate:"required" extensions:"x-nullable"`
	CommissionedDate *string  `json:"commissioned_date" example:"2019-04-30" validate:"required" extensions:"x-nullable"`
	// ChildCount is the number of immediate children (0 = leaf).
	ChildCount int `json:"child_count" example:"4" validate:"required"`
	// SubtreeCount is the number of assets in this asset's subtree,
	// including the asset itself (a leaf has 1).
	SubtreeCount int `json:"subtree_count" example:"5" validate:"required"`
}

// RootsResponse lists every top-level asset (those with no parent).
type RootsResponse struct {
	Total int         `json:"total" example:"13" validate:"required"`
	Roots []AssetNode `json:"roots" validate:"required"`
}

// TypeCount is a number of assets of one type.
type TypeCount struct {
	AssetType string `json:"asset_type" example:"TRANSFORMER" validate:"required"`
	Count     int    `json:"count" example:"3" validate:"required"`
}

// ChildGroup is the immediate children of one type.
type ChildGroup struct {
	AssetType string `json:"asset_type" example:"SWITCHBOARD" validate:"required"`
	// Count is the number of immediate children of this type.
	Count int `json:"count" example:"2" validate:"required"`
	// SubtreeCount is the combined subtree size of those children
	// (each child counted with its own descendants).
	SubtreeCount int         `json:"subtree_count" example:"9" validate:"required"`
	Assets       []AssetNode `json:"assets" validate:"required"`
}

// ChildrenResponse is the immediate children of an asset grouped by type,
// plus the count of every descendant by type ("counts per type beneath a
// substation").
type ChildrenResponse struct {
	AssetID          string       `json:"asset_id" example:"SUB-001" validate:"required"`
	TotalChildren    int          `json:"total_children" example:"6" validate:"required"`
	Groups           []ChildGroup `json:"groups" validate:"required"`
	DescendantCounts []TypeCount  `json:"descendant_counts" validate:"required"`
}

// AncestorsResponse is the path from the top of the hierarchy down to the
// asset itself, in that order (the asset is the last element).
type AncestorsResponse struct {
	AssetID string         `json:"asset_id" example:"SWP-001-1-1" validate:"required"`
	Path    []AssetSummary `json:"path" validate:"required"`
}

// SearchResponse holds matches for GET /api/assets/search.
type SearchResponse struct {
	Query string `json:"query" example:"tx-001" validate:"required"`
	// Type echoes the applied asset_type filter ("" = none).
	Type  string `json:"type" example:"TRANSFORMER" validate:"required"`
	Count int    `json:"count" example:"2" validate:"required"`
	// Truncated is true when more assets matched than limit allowed.
	Truncated bool           `json:"truncated" example:"false" validate:"required"`
	Results   []AssetSummary `json:"results" validate:"required"`
}

// AssetListResponse is one page of GET /api/assets.
type AssetListResponse struct {
	// Total is the number of assets matching the filters, across all pages.
	Total  int            `json:"total" example:"206" validate:"required"`
	Limit  int            `json:"limit" example:"25" validate:"required"`
	Offset int            `json:"offset" example:"0" validate:"required"`
	Items  []AssetSummary `json:"items" validate:"required"`
}

// StatusCount is a number of assets in one operational status.
type StatusCount struct {
	OperationalStatus string `json:"operational_status" example:"IN_SERVICE" validate:"required"`
	Count             int    `json:"count" example:"146" validate:"required"`
}

// StatsResponse summarises everything stored, for the overview.
type StatsResponse struct {
	Total    int           `json:"total" example:"206" validate:"required"`
	ByType   []TypeCount   `json:"by_type" validate:"required"`
	ByStatus []StatusCount `json:"by_status" validate:"required"`
}

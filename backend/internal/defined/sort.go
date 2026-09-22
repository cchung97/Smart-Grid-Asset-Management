package defined

// Sort keys and directions accepted by GET /api/assets (`sort` and `dir`).
// The repository maps each key to a fixed column, so a value outside this
// list is rejected rather than ever reaching the query.
const (
	SortAssetID = "asset_id"
	SortName    = "name"
	SortType    = "type"
	SortStatus  = "status"
	SortParent  = "parent"

	SortAsc  = "asc"
	SortDesc = "desc"
)

// AssetSortKeys lists the valid `sort` values, in the order the UI shows the columns.
var AssetSortKeys = []string{SortAssetID, SortName, SortType, SortStatus, SortParent}

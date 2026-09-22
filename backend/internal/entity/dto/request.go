package dto

import (
	"slices"
	"strings"

	"github.com/google/uuid"

	"smart-grid-asset-management/backend/internal/base/errx"
	"smart-grid-asset-management/backend/internal/defined"
)

// The request types below carry a request's parameters to the service, which
// calls Validate before using them: the frontend's checks are UX only, so
// every rule is enforced here regardless of what the client already did.
// Rules that need reference data (is this a known asset type?) are not
// expressible on a plain struct and stay in the service.

// SearchRequest holds the query parameters of GET /api/assets/search.
type SearchRequest struct {
	Q     string `form:"q"`
	Type  string `form:"type"`
	Limit int    `form:"limit"` // 0 means the default page size
}

// Validate normalises the request (surrounding space is trimmed from Q and
// Type) and checks it. Problems are *errx.InputError so the handler can
// answer 400.
func (r *SearchRequest) Validate() error {
	r.Q = strings.TrimSpace(r.Q)
	r.Type = strings.TrimSpace(r.Type)
	switch {
	case r.Q == "":
		return errx.InvalidInput("query parameter q is required")
	case len([]rune(r.Q)) > defined.MaxSearchQueryLen:
		return errx.InvalidInput("query parameter q must be at most %d characters", defined.MaxSearchQueryLen)
	case r.Limit < 0 || r.Limit > defined.MaxSearchSize:
		return errx.InvalidInput("limit must be between 1 and %d", defined.MaxSearchSize)
	}
	return nil
}

// ImportIDRequest identifies one past import in GET /api/imports/{importId}.
type ImportIDRequest struct {
	ImportID string `uri:"importId"`
}

// Validate checks that ImportID is a UUID.
func (r ImportIDRequest) Validate() error {
	if _, err := uuid.Parse(r.ImportID); err != nil {
		return errx.InvalidInput("import id %q is not a valid UUID", r.ImportID)
	}
	return nil
}

// ListRequest holds the query parameters of GET /api/assets.
type ListRequest struct {
	Q      string `form:"q"`
	Type   string `form:"type"`
	Status string `form:"status"`
	Sort   string `form:"sort"`  // one of defined.AssetSortKeys; "" means asset_id
	Dir    string `form:"dir"`   // asc or desc; "" means asc
	Limit  int    `form:"limit"` // 0 means the default page size
	Offset int    `form:"offset"`
}

// Validate normalises the request (surrounding space is trimmed, sort and dir
// are lower-cased and defaulted) and checks it. Type and status are matched
// against the reference tables by the service.
func (r *ListRequest) Validate() error {
	r.Q = strings.TrimSpace(r.Q)
	r.Type = strings.TrimSpace(r.Type)
	r.Status = strings.TrimSpace(r.Status)
	r.Sort = strings.ToLower(strings.TrimSpace(r.Sort))
	r.Dir = strings.ToLower(strings.TrimSpace(r.Dir))
	if r.Sort == "" {
		r.Sort = defined.SortAssetID
	}
	if r.Dir == "" {
		r.Dir = defined.SortAsc
	}
	switch {
	case !slices.Contains(defined.AssetSortKeys, r.Sort):
		return errx.InvalidInput("sort must be one of: %s", strings.Join(defined.AssetSortKeys, ", "))
	case r.Dir != defined.SortAsc && r.Dir != defined.SortDesc:
		return errx.InvalidInput("dir must be asc or desc")
	}
	switch {
	case len([]rune(r.Q)) > defined.MaxSearchQueryLen:
		return errx.InvalidInput("query parameter q must be at most %d characters", defined.MaxSearchQueryLen)
	case r.Limit < 0 || r.Limit > defined.MaxListSize:
		return errx.InvalidInput("limit must be between 1 and %d", defined.MaxListSize)
	case r.Offset < 0:
		return errx.InvalidInput("offset must not be negative")
	}
	if r.Limit == 0 {
		r.Limit = defined.DefaultListSize
	}
	return nil
}

// ImportListRequest holds the query parameters of GET /api/imports.
type ImportListRequest struct {
	Limit  int `form:"limit"` // 0 means the default page size
	Offset int `form:"offset"`
}

// Validate checks the paging parameters and fills in the default limit.
func (r *ImportListRequest) Validate() error {
	switch {
	case r.Limit < 0 || r.Limit > defined.MaxImportListSize:
		return errx.InvalidInput("limit must be between 1 and %d", defined.MaxImportListSize)
	case r.Offset < 0:
		return errx.InvalidInput("offset must not be negative")
	}
	if r.Limit == 0 {
		r.Limit = defined.DefaultImportListSize
	}
	return nil
}

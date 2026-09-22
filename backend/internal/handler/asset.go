package handler

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"smart-grid-asset-management/backend/internal/base/errx"
	"smart-grid-asset-management/backend/internal/base/httpx"
	"smart-grid-asset-management/backend/internal/base/logs"
	"smart-grid-asset-management/backend/internal/entity/dto"
)

type assetService interface {
	Roots(ctx context.Context) (dto.RootsResponse, error)
	Get(ctx context.Context, id string) (dto.AssetDetail, error)
	Children(ctx context.Context, id string) (dto.ChildrenResponse, error)
	Ancestors(ctx context.Context, id string) (dto.AncestorsResponse, error)
	Search(ctx context.Context, req dto.SearchRequest) (dto.SearchResponse, error)
	List(ctx context.Context, req dto.ListRequest) (dto.AssetListResponse, error)
	Stats(ctx context.Context) (dto.StatsResponse, error)
	Delete(ctx context.Context, id string) error
}

// AssetHandler serves the asset hierarchy, search, list and stats endpoints and
// the delete endpoint.
type AssetHandler struct{ service assetService }

func NewAssetHandler(s assetService) *AssetHandler { return &AssetHandler{service: s} }

// Roots godoc
// @Summary      List top-level assets
// @Description  Every asset that has no parent (substations), each with `child_count` and `subtree_count` so a tree can render expanders without further requests.
// @Tags         assets
// @Produce      json
// @Success      200  {object}  dto.RootsResponse
// @Failure      500  {object}  dto.ErrorResponse
// @Failure      401  {object}  dto.ErrorResponse  "Missing or invalid x-api-key"
// @Security     ApiKeyAuth
// @Router       /api/assets/roots [get]
func (h *AssetHandler) Roots(c *gin.Context) {
	resp, err := h.service.Roots(c.Request.Context())
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

// Get godoc
// @Summary      Get one asset
// @Description  Every stored attribute of the asset.
// @Tags         assets
// @Produce      json
// @Param        assetId  path      string  true  "Asset id"  example(TX-001-1)
// @Success      200  {object}  dto.AssetDetail
// @Failure      404  {object}  dto.ErrorResponse  "No such asset"
// @Failure      500  {object}  dto.ErrorResponse
// @Failure      401  {object}  dto.ErrorResponse  "Missing or invalid x-api-key"
// @Security     ApiKeyAuth
// @Router       /api/assets/{assetId} [get]
func (h *AssetHandler) Get(c *gin.Context) {
	resp, err := h.service.Get(c.Request.Context(), c.Param("assetId"))
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

// Children godoc
// @Summary      List an asset's children
// @Description  The asset's immediate children grouped by asset type (each with `child_count` and `subtree_count`), plus `descendant_counts`: how many descendants of each type sit anywhere beneath the asset. An asset with no children returns empty lists.
// @Tags         assets
// @Produce      json
// @Param        assetId  path      string  true  "Asset id"  example(SUB-001)
// @Success      200  {object}  dto.ChildrenResponse
// @Failure      404  {object}  dto.ErrorResponse  "No such asset"
// @Failure      500  {object}  dto.ErrorResponse
// @Failure      401  {object}  dto.ErrorResponse  "Missing or invalid x-api-key"
// @Security     ApiKeyAuth
// @Router       /api/assets/{assetId}/children [get]
func (h *AssetHandler) Children(c *gin.Context) {
	resp, err := h.service.Children(c.Request.Context(), c.Param("assetId"))
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

// Ancestors godoc
// @Summary      Get an asset's ancestor path
// @Description  The path from the top of the hierarchy down to the asset, in that order; the asset itself is the last element. Use it to expand a tree to a search hit.
// @Tags         assets
// @Produce      json
// @Param        assetId  path      string  true  "Asset id"  example(SWP-001-1-1)
// @Success      200  {object}  dto.AncestorsResponse
// @Failure      404  {object}  dto.ErrorResponse  "No such asset"
// @Failure      500  {object}  dto.ErrorResponse
// @Failure      401  {object}  dto.ErrorResponse  "Missing or invalid x-api-key"
// @Security     ApiKeyAuth
// @Router       /api/assets/{assetId}/ancestors [get]
func (h *AssetHandler) Ancestors(c *gin.Context) {
	resp, err := h.service.Ancestors(c.Request.Context(), c.Param("assetId"))
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

// Search godoc
// @Summary      Search assets
// @Description  Case-insensitive substring match on asset id or name, optionally restricted to one asset type. Exact id matches come first, then id prefixes. `%` and `_` in `q` are matched literally. Results are capped at `limit`; `truncated` says whether more matched.
// @Tags         assets
// @Produce      json
// @Param        q      query     string  true   "Text to look for in the asset id or name"  minlength(1) maxlength(200)
// @Param        type   query     string  false  "Only assets of this type (see GET /api/lookups)"
// @Param        limit  query     int     false  "Maximum results"  default(50) minimum(1) maximum(200)
// @Success      200  {object}  dto.SearchResponse
// @Failure      400  {object}  dto.ErrorResponse  "Missing q, unknown type, or bad limit"
// @Failure      500  {object}  dto.ErrorResponse
// @Failure      401  {object}  dto.ErrorResponse  "Missing or invalid x-api-key"
// @Security     ApiKeyAuth
// @Router       /api/assets/search [get]
func (h *AssetHandler) Search(c *gin.Context) {
	var req dto.SearchRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		writeError(c, badQuery(err))
		return
	}
	resp, err := h.service.Search(c.Request.Context(), req)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

// List godoc
// @Summary      List all assets
// @Description  One page of every stored asset, ordered by asset id unless `sort` says otherwise, optionally filtered by text (case-insensitive substring of the id or name), asset type and operational status. `total` counts all matches across pages.
// @Tags         assets
// @Produce      json
// @Param        q       query     string  false  "Text to look for in the asset id or name"  maxlength(200)
// @Param        type    query     string  false  "Only assets of this type (see GET /api/lookups)"
// @Param        status  query     string  false  "Only assets in this operational status (see GET /api/lookups)"
// @Param        sort    query     string  false  "Column to order by; ties and the default are by asset id. Assets with no parent sort last."  Enums(asset_id, name, type, status, parent)  default(asset_id)
// @Param        dir     query     string  false  "Sort direction"  Enums(asc, desc)  default(asc)
// @Param        limit   query     int     false  "Page size"  default(25) minimum(1) maximum(200)
// @Param        offset  query     int     false  "Rows to skip"  default(0) minimum(0)
// @Success      200  {object}  dto.AssetListResponse
// @Failure      400  {object}  dto.ErrorResponse  "Unknown type, status or sort, or bad dir/limit/offset"
// @Failure      500  {object}  dto.ErrorResponse
// @Failure      401  {object}  dto.ErrorResponse  "Missing or invalid x-api-key"
// @Security     ApiKeyAuth
// @Router       /api/assets [get]
func (h *AssetHandler) List(c *gin.Context) {
	var req dto.ListRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		writeError(c, errx.InvalidInput("limit and offset must be integers"))
		return
	}
	resp, err := h.service.List(c.Request.Context(), req)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

// Stats godoc
// @Summary      Count assets by type and status
// @Description  The total number of stored assets and how many there are of each asset type and each operational status. Types and statuses with no assets are omitted.
// @Tags         assets
// @Produce      json
// @Success      200  {object}  dto.StatsResponse
// @Failure      500  {object}  dto.ErrorResponse
// @Failure      401  {object}  dto.ErrorResponse  "Missing or invalid x-api-key"
// @Security     ApiKeyAuth
// @Router       /api/assets/stats [get]
func (h *AssetHandler) Stats(c *gin.Context) {
	resp, err := h.service.Stats(c.Request.Context())
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

// Delete godoc
// @Summary      Delete one asset
// @Description  Removes an asset that has no children. An asset that still has children is refused with 409 and must have them deleted first. See docs/KNOWN_LIMITATIONS.md.
// @Tags         assets
// @Security     ApiKeyAuth
// @Param        assetId  path  string  true  "Asset id"  example(TX-001-1)
// @Success      204  "Deleted"
// @Failure      401  {object}  dto.ErrorResponse  "Missing or invalid x-api-key"
// @Failure      403  {object}  dto.ErrorResponse  "No API key is configured on the server, so the API is disabled"
// @Failure      404  {object}  dto.ErrorResponse  "No such asset"
// @Failure      409  {object}  dto.ErrorResponse  "The asset has children, or another writer collided with the delete"
// @Failure      500  {object}  dto.ErrorResponse
// @Router       /api/assets/{assetId} [delete]
func (h *AssetHandler) Delete(c *gin.Context) {
	id := c.Param("assetId")
	if err := h.service.Delete(c.Request.Context(), id); err != nil {
		writeError(c, err)
		return
	}
	logs.WithCtx(c.Request.Context()).Info("asset deleted", "asset_id", id, "client_ip", httpx.ClientIP(c.Request))
	c.Status(http.StatusNoContent)
	c.Writer.WriteHeaderNow()
}

// badQuery turns a query-binding failure into a client-correctable error.
// The only numeric parameter is limit, so a parse failure is always that.
func badQuery(err error) error {
	var numErr *strconv.NumError
	if errors.As(err, &numErr) {
		return errx.InvalidInput("limit must be an integer")
	}
	return errx.InvalidInput("invalid query parameters")
}

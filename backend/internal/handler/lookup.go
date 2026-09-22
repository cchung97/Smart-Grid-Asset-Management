package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"smart-grid-asset-management/backend/internal/service"
)

type snapshotSource interface {
	Snapshot() *service.LookupSnapshot
}

// LookupHandler serves the reference data validation runs against.
type LookupHandler struct{ source snapshotSource }

func NewLookupHandler(s snapshotSource) *LookupHandler { return &LookupHandler{source: s} }

// List godoc
// @Summary      List reference data
// @Description  The asset types, operational statuses and parent/child type pairings that imports are validated against, read from the database (refreshed on startup and on every import). Use it to build filters and dropdowns instead of hardcoding values.
// @Tags         lookups
// @Produce      json
// @Success      200  {object}  dto.LookupsResponse
// @Failure      401  {object}  dto.ErrorResponse  "Missing or invalid x-api-key"
// @Security     ApiKeyAuth
// @Router       /api/lookups [get]
func (h *LookupHandler) List(c *gin.Context) {
	c.JSON(http.StatusOK, h.source.Snapshot().LookupsResponse())
}

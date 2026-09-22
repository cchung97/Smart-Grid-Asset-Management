// Package handler adapts gin requests to service calls: parse the
// request into a dto, call a service, write the dto response as JSON.
// Handlers hold no business logic and no SQL — both belong one layer
// down, in service and repository respectively.
package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"smart-grid-asset-management/backend/internal/service"
)

type HealthHandler struct {
	service *service.HealthService
}

func NewHealthHandler(s *service.HealthService) *HealthHandler {
	return &HealthHandler{service: s}
}

// Healthz godoc
// @Summary      Health check
// @Description  Returns service liveness and DB connectivity status.
// @Tags         health
// @Produce      json
// @Success      200  {object}  dto.HealthResponse
// @Failure      503  {object}  dto.HealthResponse
// @Router       /healthz [get]
func (h *HealthHandler) Healthz(c *gin.Context) {
	resp := h.service.Check(c.Request.Context())
	status := http.StatusOK
	if resp.Status != "ok" {
		status = http.StatusServiceUnavailable
	}
	c.JSON(status, resp)
}

// Package service holds business logic: CSV validation, the two-pass
// hierarchy algorithm, transactional import commit, search, etc. (see
// ARCHITECTURE.md section 4). Services are called by handlers, call
// repositories, and convert between entity/model and entity/dto types.
// This is where all re-validation happens — handlers must never trust
// client input directly.
package service

import (
	"context"
	"time"

	"gorm.io/gorm"

	"smart-grid-asset-management/backend/internal/entity/dto"
)

// HealthService reports liveness plus DB connectivity.
type HealthService struct {
	db *gorm.DB
}

func NewHealthService(db *gorm.DB) *HealthService {
	return &HealthService{db: db}
}

func (s *HealthService) Check(ctx context.Context) dto.HealthResponse {
	if s.db == nil {
		return dto.HealthResponse{Status: "ok", DB: "not_configured"}
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	sqlDB, err := s.db.DB()
	if err != nil || sqlDB.PingContext(ctx) != nil {
		return dto.HealthResponse{Status: "degraded", DB: "down"}
	}
	return dto.HealthResponse{Status: "ok", DB: "up"}
}

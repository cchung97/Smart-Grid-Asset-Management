package repository

import (
	"context"
	"fmt"

	"gorm.io/gorm"

	"smart-grid-asset-management/backend/internal/entity/model"
)

// LookupRepository reads the reference tables validation runs against.
type LookupRepository struct{ db *gorm.DB }

func NewLookupRepository(db *gorm.DB) *LookupRepository { return &LookupRepository{db: db} }

// AssetTypes returns every asset_types code, sorted.
func (r *LookupRepository) AssetTypes(ctx context.Context) ([]string, error) {
	var codes []string
	if err := r.db.WithContext(ctx).Model(&model.AssetType{}).Order("code").Pluck("code", &codes).Error; err != nil {
		return nil, fmt.Errorf("query %s: %w", model.AssetType{}.TableName(), err)
	}
	return codes, nil
}

// OperationalStatuses returns every operational_statuses code, sorted.
func (r *LookupRepository) OperationalStatuses(ctx context.Context) ([]string, error) {
	var codes []string
	if err := r.db.WithContext(ctx).Model(&model.OperationalStatus{}).Order("code").Pluck("code", &codes).Error; err != nil {
		return nil, fmt.Errorf("query %s: %w", model.OperationalStatus{}.TableName(), err)
	}
	return codes, nil
}

// ParentRules returns every permitted child->parent type pair.
func (r *LookupRepository) ParentRules(ctx context.Context) ([]model.ParentRule, error) {
	var rules []model.ParentRule
	if err := r.db.WithContext(ctx).Order("child_type_code, parent_type_code").Find(&rules).Error; err != nil {
		return nil, fmt.Errorf("query %s: %w", model.ParentRule{}.TableName(), err)
	}
	return rules, nil
}

package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"smart-grid-asset-management/backend/internal/base/errx"
	"smart-grid-asset-management/backend/internal/entity/model"
)

// rejectionChunkRows keeps one multi-row INSERT far below the parameter limit (4 per row).
const rejectionChunkRows = 1000

// runColumns reads client_ip through host(): it is an INET column, and the
// model holds the bare address as a string.
const runColumns = "id, filename, host(client_ip) AS client_ip, total_rows, imported_rows, rejected_rows, committed, created_at, updated_at"

// ImportRepository reads and writes the import_runs / import_rejections audit tables.
type ImportRepository struct{ db *gorm.DB }

func NewImportRepository(db *gorm.DB) *ImportRepository { return &ImportRepository{db: db} }

// CreateRun inserts the audit row for one upload.
func (r *ImportRepository) CreateRun(ctx context.Context, run model.ImportRun) error {
	if err := r.db.WithContext(ctx).Create(&run).Error; err != nil {
		return fmt.Errorf("insert import_run: %w", err)
	}
	return nil
}

// CreateRejections inserts the per-row rejections of one upload, in chunks.
func (r *ImportRepository) CreateRejections(ctx context.Context, runID uuid.UUID, rejections []model.ImportRejection) error {
	for start := 0; start < len(rejections); start += rejectionChunkRows {
		end := min(start+rejectionChunkRows, len(rejections))
		chunk := make([]model.ImportRejection, end-start)
		copy(chunk, rejections[start:end])
		for i := range chunk {
			chunk[i].ImportRunID = runID
		}
		if err := r.db.WithContext(ctx).Create(&chunk).Error; err != nil {
			return fmt.Errorf("insert %s (rows %d-%d): %w", model.ImportRejection{}.TableName(), start+1, end, err)
		}
	}
	return nil
}

// List returns one page of import runs, newest first, plus how many exist.
func (r *ImportRepository) List(ctx context.Context, limit, offset int) ([]model.ImportRun, int64, error) {
	var total int64
	if err := r.db.WithContext(ctx).Model(&model.ImportRun{}).Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count import_runs: %w", err)
	}
	var out []model.ImportRun
	err := r.db.WithContext(ctx).Select(runColumns).
		Order("created_at DESC, id").Limit(limit).Offset(offset).Find(&out).Error
	if err != nil {
		return nil, 0, fmt.Errorf("list import_runs: %w", err)
	}
	return out, total, nil
}

// GetRun returns one audit row, or errx.ErrNotFound.
func (r *ImportRepository) GetRun(ctx context.Context, id uuid.UUID) (model.ImportRun, error) {
	var run model.ImportRun
	err := r.db.WithContext(ctx).Select(runColumns).Where("id = ?", id).First(&run).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return model.ImportRun{}, errx.ErrNotFound
	}
	if err != nil {
		return model.ImportRun{}, fmt.Errorf("get import_run %s: %w", id, err)
	}
	return run, nil
}

// ListRejections returns a run's rejections in file order.
func (r *ImportRepository) ListRejections(ctx context.Context, runID uuid.UUID) ([]model.ImportRejection, error) {
	var out []model.ImportRejection
	err := r.db.WithContext(ctx).Where("import_run_id = ?", runID).Order("csv_row_number, id").Find(&out).Error
	if err != nil {
		return nil, fmt.Errorf("query %s: %w", model.ImportRejection{}.TableName(), err)
	}
	return out, nil
}

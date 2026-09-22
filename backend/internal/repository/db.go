package repository

import (
	"context"
	"errors"
	"fmt"

	"gorm.io/gorm"

	"smart-grid-asset-management/backend/internal/base/errx"
)

// InTx runs fn inside a transaction: committed if fn returns nil, rolled
// back if it returns an error or panics. fn must build its repositories
// over the tx it is handed.
func InTx(ctx context.Context, db *gorm.DB, fn func(tx *gorm.DB) error) error {
	return mapWriteError(db.WithContext(ctx).Transaction(fn))
}

// mapWriteError wraps unique/foreign-key violations (which the connection's
// TranslateError setting surfaces as gorm errors) as errx.ErrConflict and
// passes every other error through unchanged.
func mapWriteError(err error) error {
	if errors.Is(err, gorm.ErrDuplicatedKey) || errors.Is(err, gorm.ErrForeignKeyViolated) {
		return fmt.Errorf("%w: %v", errx.ErrConflict, err)
	}
	return err
}

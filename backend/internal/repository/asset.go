package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/lib/pq"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"smart-grid-asset-management/backend/internal/base/errx"
	"smart-grid-asset-management/backend/internal/defined"
	"smart-grid-asset-management/backend/internal/entity/model"
)

// maxAncestorDepth bounds the upward walk so a corrupted parent cycle
// cannot recurse forever.
const maxAncestorDepth = 64

// insertChunkRows keeps one multi-row INSERT far below Postgres's
// 65535-parameter limit (11 parameters per asset).
const insertChunkRows = 500

// assetsTable is the table the recursive CTEs read; it comes from the
// model so a rename happens in one place.
var assetsTable = model.Asset{}.TableName()

var assetColumnNames = []string{
	"asset_id", "parent_asset_id", "asset_type", "asset_name", "operational_status",
	"voltage_kv", "rating_kva", "manufacturer", "model", "serial_number",
	"commissioned_date", "created_at", "updated_at",
}

// assetCols renders the asset column list, optionally qualified by a table
// alias, for the raw recursive queries.
func assetCols(alias string) string {
	parts := make([]string, len(assetColumnNames))
	for i, c := range assetColumnNames {
		if alias != "" {
			parts[i] = alias + "." + c
		} else {
			parts[i] = c
		}
	}
	return strings.Join(parts, ", ")
}

// AssetRepository reads and writes the assets table.
type AssetRepository struct{ db *gorm.DB }

func NewAssetRepository(db *gorm.DB) *AssetRepository { return &AssetRepository{db: db} }

// Get returns one asset, or errx.ErrNotFound.
func (r *AssetRepository) Get(ctx context.Context, id string) (model.Asset, error) {
	var a model.Asset
	err := r.db.WithContext(ctx).Where("asset_id = ?", id).First(&a).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return model.Asset{}, errx.ErrNotFound
	}
	if err != nil {
		return model.Asset{}, fmt.Errorf("get asset %q: %w", id, err)
	}
	return a, nil
}

// Exists reports whether an asset with this id is stored.
func (r *AssetRepository) Exists(ctx context.Context, id string) (bool, error) {
	var n int64
	if err := r.db.WithContext(ctx).Model(&model.Asset{}).Where("asset_id = ?", id).Limit(1).Count(&n).Error; err != nil {
		return false, fmt.Errorf("check asset %q: %w", id, err)
	}
	return n > 0, nil
}

// ExistingByIDs returns asset_id -> asset_type for whichever of ids are
// already stored. One round trip regardless of len(ids).
//
// The ids travel as a single array parameter (= ANY(?)) rather than an IN
// list: GORM expands a slice into one placeholder per element, and a large
// file (rows are unbounded by design) would exceed Postgres's 65535
// parameter limit. pq.Array is a driver.Valuer, which GORM binds as one value.
func (r *AssetRepository) ExistingByIDs(ctx context.Context, ids []string) (map[string]string, error) {
	out := make(map[string]string, len(ids))
	if len(ids) == 0 {
		return out, nil
	}
	var found []model.Asset
	err := r.db.WithContext(ctx).
		Select("asset_id", "asset_type").
		Where("asset_id = ANY(?)", pq.Array(ids)).
		Find(&found).Error
	if err != nil {
		return nil, fmt.Errorf("query existing assets: %w", err)
	}
	for _, a := range found {
		out[a.AssetID] = a.AssetType
	}
	return out, nil
}

// InsertBatch inserts assets in the order given, in chunked multi-row
// statements. Callers must order parents before children: assets.
// parent_asset_id is a plain (non-deferrable) foreign key, and rows in a
// later chunk may reference rows in an earlier one. Run it on a
// transaction-bound repository so the whole import commits or none does.
func (r *AssetRepository) InsertBatch(ctx context.Context, assets []model.Asset) error {
	for start := 0; start < len(assets); start += insertChunkRows {
		end := min(start+insertChunkRows, len(assets))
		chunk := make([]model.Asset, end-start)
		copy(chunk, assets[start:end])
		for i := range chunk {
			chunk[i].CommissionedDate = dateOnly(chunk[i].CommissionedDate)
		}
		if err := r.db.WithContext(ctx).Create(&chunk).Error; err != nil {
			return fmt.Errorf("insert assets (rows %d-%d): %w", start+1, end, mapWriteError(err))
		}
	}
	return nil
}

// dateOnly normalises to a UTC calendar date so a DATE column never
// depends on the server's time zone.
func dateOnly(t *time.Time) *time.Time {
	if t == nil {
		return nil
	}
	y, m, d := t.Date()
	n := time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
	return &n
}

const (
	condRoots    = "parent_asset_id IS NULL"
	condChildren = "parent_asset_id = ?"
)

// ListWithCounts returns the immediate children of parentID (or, when
// parentID is nil, every top-level asset), each with its child count and
// subtree size, ordered by type then id. One recursive query for all
// subtree sizes; UNION (not UNION ALL) so a corrupt cycle terminates.
func (r *AssetRepository) ListWithCounts(ctx context.Context, parentID *string) ([]model.AssetWithCounts, error) {
	cond, args := condRoots, []any{}
	if parentID != nil {
		// The condition appears twice in the query, once per reference.
		cond, args = condChildren, []any{*parentID, *parentID}
	}
	query := fmt.Sprintf(`
WITH RECURSIVE sub(root_id, node_id) AS (
    SELECT asset_id, asset_id FROM %[3]s WHERE %[1]s
  UNION
    SELECT s.root_id, a.asset_id FROM sub s JOIN %[3]s a ON a.parent_asset_id = s.node_id
)
SELECT %[2]s,
       (SELECT count(*) FROM %[3]s c WHERE c.parent_asset_id = a.asset_id) AS child_count,
       s.n AS subtree_count
FROM %[3]s a
JOIN (SELECT root_id, count(*) AS n FROM sub GROUP BY root_id) s ON s.root_id = a.asset_id
WHERE a.%[1]s
ORDER BY a.asset_type, a.asset_id`, cond, assetCols("a"), assetsTable)

	var out []model.AssetWithCounts
	if err := r.db.WithContext(ctx).Raw(query, args...).Scan(&out).Error; err != nil {
		return nil, fmt.Errorf("list assets with counts: %w", err)
	}
	return out, nil
}

// DescendantCounts returns how many descendants of id exist per asset type.
func (r *AssetRepository) DescendantCounts(ctx context.Context, id string) ([]model.TypeCount, error) {
	query := fmt.Sprintf(`
WITH RECURSIVE d(asset_id) AS (
    SELECT asset_id FROM %[1]s WHERE parent_asset_id = ?
  UNION
    SELECT a.asset_id FROM d JOIN %[1]s a ON a.parent_asset_id = d.asset_id
)
SELECT a.asset_type, count(*) AS count FROM d JOIN %[1]s a USING (asset_id)
GROUP BY a.asset_type ORDER BY a.asset_type`, assetsTable)

	var out []model.TypeCount
	if err := r.db.WithContext(ctx).Raw(query, id).Scan(&out).Error; err != nil {
		return nil, fmt.Errorf("count descendants of %q: %w", id, err)
	}
	return out, nil
}

// Ancestors returns the path from the top of the hierarchy down to the
// asset itself (root first, the asset last), or errx.ErrNotFound.
func (r *AssetRepository) Ancestors(ctx context.Context, id string) ([]model.Asset, error) {
	query := fmt.Sprintf(`
WITH RECURSIVE up AS (
    SELECT %[1]s, 0 AS depth FROM %[4]s WHERE asset_id = ?
  UNION ALL
    SELECT %[2]s, up.depth + 1 FROM %[4]s a JOIN up ON a.asset_id = up.parent_asset_id WHERE up.depth < %[3]d
)
SELECT %[1]s FROM up ORDER BY depth DESC`, assetCols(""), assetCols("a"), maxAncestorDepth, assetsTable)

	var out []model.Asset
	if err := r.db.WithContext(ctx).Raw(query, id).Scan(&out).Error; err != nil {
		return nil, fmt.Errorf("query ancestors of %q: %w", id, err)
	}
	if len(out) == 0 {
		return nil, errx.ErrNotFound
	}
	return out, nil
}

// Search returns assets whose id or name contains q (case-insensitive),
// optionally restricted to one asset type ("" = any), best matches first:
// exact id, then id prefix, then the rest by id. LIKE wildcards in q are
// matched literally. The trigram GIN indexes on asset_id/asset_name serve
// the substring match.
func (r *AssetRepository) Search(ctx context.Context, q, assetType string, limit int) ([]model.Asset, error) {
	esc := escapeLike(q)
	contains := "%" + esc + "%"

	tx := r.db.WithContext(ctx).Model(&model.Asset{}).
		Where(`(asset_id ILIKE ? ESCAPE '\' OR asset_name ILIKE ? ESCAPE '\')`, contains, contains)
	if assetType != "" {
		tx = tx.Where("asset_type = ?", assetType)
	}
	var out []model.Asset
	err := tx.
		Clauses(clause.OrderBy{Expression: clause.Expr{
			SQL:                `(lower(asset_id) = lower(?)) DESC, (asset_id ILIKE ? ESCAPE '\') DESC, asset_id`,
			Vars:               []any{q, esc + "%"},
			WithoutParentheses: true,
		}}).
		Limit(limit).
		Find(&out).Error
	if err != nil {
		return nil, fmt.Errorf("search assets: %w", err)
	}
	return out, nil
}

// AssetFilter narrows List. Empty fields do not filter.
type AssetFilter struct {
	Q      string // case-insensitive substring of the id or name (LIKE wildcards are literal)
	Type   string
	Status string
	Sort   string // one of defined.AssetSortKeys; "" means asset_id
	Desc   bool
	Limit  int
	Offset int
}

// assetOrder maps each sort key to the SQL it orders by. The keys come from
// the request but the SQL never does: an unknown key falls back to asset_id.
var assetOrder = map[string]string{
	defined.SortAssetID: "asset_id",
	defined.SortName:    "lower(asset_name)",
	defined.SortType:    "asset_type",
	defined.SortStatus:  "operational_status",
	defined.SortParent:  "parent_asset_id",
}

// orderClause is the ORDER BY for a sort key and direction. Assets with no
// parent go last whichever way parents are sorted, and asset_id breaks ties so
// a page boundary never shuffles rows between pages.
func orderClause(sort string, desc bool) string {
	col, ok := assetOrder[sort]
	if !ok {
		sort, col = defined.SortAssetID, assetOrder[defined.SortAssetID]
	}
	dir := "ASC"
	if desc {
		dir = "DESC"
	}
	clause := col + " " + dir
	if sort == defined.SortParent {
		clause += " NULLS LAST"
	}
	if sort != defined.SortAssetID {
		clause += ", asset_id"
	}
	return clause
}

// List returns one page of assets in the requested order (default: by id),
// plus how many match the filter in total.
func (r *AssetRepository) List(ctx context.Context, f AssetFilter) ([]model.Asset, int64, error) {
	tx := r.db.WithContext(ctx).Model(&model.Asset{})
	if f.Q != "" {
		contains := "%" + escapeLike(f.Q) + "%"
		tx = tx.Where(`(asset_id ILIKE ? ESCAPE '\' OR asset_name ILIKE ? ESCAPE '\')`, contains, contains)
	}
	if f.Type != "" {
		tx = tx.Where("asset_type = ?", f.Type)
	}
	if f.Status != "" {
		tx = tx.Where("operational_status = ?", f.Status)
	}
	var total int64
	if err := tx.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count assets: %w", err)
	}
	var out []model.Asset
	if err := tx.Order(orderClause(f.Sort, f.Desc)).Limit(f.Limit).Offset(f.Offset).Find(&out).Error; err != nil {
		return nil, 0, fmt.Errorf("list assets: %w", err)
	}
	return out, total, nil
}

// CountByType returns how many assets exist per asset type.
func (r *AssetRepository) CountByType(ctx context.Context) ([]model.TypeCount, error) {
	var out []model.TypeCount
	err := r.db.WithContext(ctx).Model(&model.Asset{}).
		Select("asset_type, count(*) AS count").Group("asset_type").Order("asset_type").Scan(&out).Error
	if err != nil {
		return nil, fmt.Errorf("count assets by type: %w", err)
	}
	return out, nil
}

// CountByStatus returns how many assets exist per operational status.
func (r *AssetRepository) CountByStatus(ctx context.Context) ([]model.StatusCount, error) {
	var out []model.StatusCount
	err := r.db.WithContext(ctx).Model(&model.Asset{}).
		Select("operational_status, count(*) AS count").Group("operational_status").Order("operational_status").Scan(&out).Error
	if err != nil {
		return nil, fmt.Errorf("count assets by status: %w", err)
	}
	return out, nil
}

// Delete removes one asset that has no children. It returns errx.ErrNotFound
// for an unknown id and *errx.ConflictError (with the child count) if the asset
// still has children: the caller must delete those first. The check and the
// delete share a transaction, and the parent foreign key is the last guard
// against a child being added in between (that surfaces as errx.ErrConflict).
func (r *AssetRepository) Delete(ctx context.Context, id string) error {
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var children int64
		if err := tx.Model(&model.Asset{}).Where("parent_asset_id = ?", id).Count(&children).Error; err != nil {
			return fmt.Errorf("count children of %q: %w", id, err)
		}
		if children > 0 {
			return errx.Conflict("asset %q has %d child asset(s); delete them first", id, children)
		}
		res := tx.Where("asset_id = ?", id).Delete(&model.Asset{})
		if res.Error != nil {
			return fmt.Errorf("delete asset %q: %w", id, res.Error)
		}
		if res.RowsAffected == 0 {
			return errx.ErrNotFound
		}
		return nil
	})
	return mapWriteError(err)
}

var likeEscaper = strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)

func escapeLike(s string) string { return likeEscaper.Replace(s) }

// Package model holds the domain/persistence model: plain structs that
// mirror the database schema in db/migrations, tagged for GORM, plus the
// helpers that project a model onto its API shape (see convert.go). Models
// have no JSON tags and no business rules — they are what repositories read
// and write. The API wire types themselves live in package entity/dto,
// which must never import this package.
package model

import "time"

// Asset mirrors the assets table (db/migrations/000001_init.up.sql).
//
// AssetType and OperationalStatus are plain strings on purpose: the
// permitted values live in the asset_types / operational_statuses lookup
// tables, and validation reads an in-memory snapshot of them (see
// service.LookupCache), so adding a type or status is a data change, not
// a code change.
//
// CreatedAt/UpdatedAt are read-only to GORM (`<-:false`): the database owns
// them (DEFAULT now() and the set_updated_at trigger), so an insert never
// stamps the application clock over them.
type Asset struct {
	AssetID           string `gorm:"primaryKey"`
	ParentAssetID     *string
	AssetType         string
	AssetName         string
	OperationalStatus string
	VoltageKV         *float64
	RatingKVA         *float64
	Manufacturer      *string
	Model             *string
	SerialNumber      *string
	CommissionedDate  *time.Time
	CreatedAt         time.Time `gorm:"<-:false"`
	UpdatedAt         time.Time `gorm:"<-:false"`
}

// TableName is the table Asset mirrors.
func (Asset) TableName() string { return "assets" }

// AssetWithCounts is an asset plus the two figures an explorer tree needs:
// how many immediate children it has and how large its subtree is. It is
// a query projection, not a table; it inherits Asset's TableName only
// because it embeds it.
type AssetWithCounts struct {
	Asset        `gorm:"embedded"`
	ChildCount   int
	SubtreeCount int // includes the asset itself
}

// TypeCount is a number of assets of one type. It is a query projection
// (a GROUP BY result), not a table.
type TypeCount struct {
	AssetType string
	Count     int
}

// StatusCount is a number of assets in one operational status (a GROUP BY
// result, not a table).
type StatusCount struct {
	OperationalStatus string
	Count             int
}

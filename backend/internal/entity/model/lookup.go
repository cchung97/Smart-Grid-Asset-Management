package model

// AssetType mirrors the asset_types lookup table: one row per permitted
// asset type code.
type AssetType struct {
	Code string `gorm:"primaryKey"`
}

// TableName is the table AssetType mirrors.
func (AssetType) TableName() string { return "asset_types" }

// OperationalStatus mirrors the operational_statuses lookup table: one row
// per permitted operational status code.
type OperationalStatus struct {
	Code string `gorm:"primaryKey"`
}

// TableName is the table OperationalStatus mirrors.
func (OperationalStatus) TableName() string { return "operational_statuses" }

// ParentRule mirrors asset_type_parent_rules: a child type may sit directly
// beneath a parent type.
type ParentRule struct {
	ChildType  string `gorm:"column:child_type_code;primaryKey"`
	ParentType string `gorm:"column:parent_type_code;primaryKey"`
}

// TableName is the table ParentRule mirrors.
func (ParentRule) TableName() string { return "asset_type_parent_rules" }

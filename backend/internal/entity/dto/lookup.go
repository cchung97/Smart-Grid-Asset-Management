package dto

// ParentRule states that a child asset type may sit directly beneath a
// parent asset type.
type ParentRule struct {
	ChildType  string `json:"child_type" example:"TRANSFORMER" validate:"required"`
	ParentType string `json:"parent_type" example:"SUBSTATION" validate:"required"`
}

// LookupsResponse exposes the reference data validation runs against, so
// clients can build filters and dropdowns without hardcoding it.
type LookupsResponse struct {
	AssetTypes          []string     `json:"asset_types" example:"SUBSTATION,TRANSFORMER" validate:"required"`
	OperationalStatuses []string     `json:"operational_statuses" example:"IN_SERVICE,MAINTENANCE" validate:"required"`
	ParentRules         []ParentRule `json:"parent_rules" validate:"required"`
	// RootTypes are asset types with no parent rule: they must have no parent.
	RootTypes []string `json:"root_types" example:"SUBSTATION" validate:"required"`
}

package model

import (
	"time"

	"smart-grid-asset-management/backend/internal/defined"
	"smart-grid-asset-management/backend/internal/entity/dto"
)

// Projections of a model onto its API shape. Keeping them on the model
// means every service that returns an asset renders it the same way.

// ToSummary is the compact form used in lists, paths and search results.
func (a Asset) ToSummary() dto.AssetSummary {
	return dto.AssetSummary{
		AssetID:           a.AssetID,
		ParentAssetID:     a.ParentAssetID,
		AssetType:         a.AssetType,
		AssetName:         a.AssetName,
		OperationalStatus: a.OperationalStatus,
	}
}

// ToDetail is every stored attribute of the asset.
func (a Asset) ToDetail() dto.AssetDetail {
	d := dto.AssetDetail{
		AssetID:           a.AssetID,
		ParentAssetID:     a.ParentAssetID,
		AssetType:         a.AssetType,
		AssetName:         a.AssetName,
		OperationalStatus: a.OperationalStatus,
		VoltageKV:         a.VoltageKV,
		RatingKVA:         a.RatingKVA,
		Manufacturer:      a.Manufacturer,
		Model:             a.Model,
		SerialNumber:      a.SerialNumber,
		CreatedAt:         a.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:         a.UpdatedAt.UTC().Format(time.RFC3339),
	}
	if a.CommissionedDate != nil {
		s := a.CommissionedDate.Format(defined.DateFormat)
		d.CommissionedDate = &s
	}
	return d
}

// ToNode is the asset plus what the explorer tree needs to render it.
func (c AssetWithCounts) ToNode() dto.AssetNode {
	n := dto.AssetNode{
		AssetID:           c.AssetID,
		ParentAssetID:     c.ParentAssetID,
		AssetType:         c.AssetType,
		AssetName:         c.AssetName,
		OperationalStatus: c.OperationalStatus,
		RatingKVA:         c.RatingKVA,
		ChildCount:        c.ChildCount,
		SubtreeCount:      c.SubtreeCount,
	}
	if c.CommissionedDate != nil {
		s := c.CommissionedDate.Format(defined.DateFormat)
		n.CommissionedDate = &s
	}
	return n
}

func (t TypeCount) ToDTO() dto.TypeCount {
	return dto.TypeCount{AssetType: t.AssetType, Count: t.Count}
}

func (s StatusCount) ToDTO() dto.StatusCount {
	return dto.StatusCount{OperationalStatus: s.OperationalStatus, Count: s.Count}
}

func (r ParentRule) ToDTO() dto.ParentRule {
	return dto.ParentRule{ChildType: r.ChildType, ParentType: r.ParentType}
}

// ToDTO renders a stored rejection; a rejection with no asset id (the row
// had none) reports it as "".
func (r ImportRejection) ToDTO() dto.Rejection {
	id := ""
	if r.AssetID != nil {
		id = *r.AssetID
	}
	return dto.Rejection{CSVRow: r.CSVRowNumber, AssetID: id, Reason: r.Reason}
}

// ToResponse renders the run as an import response. ignored lists header
// columns the importer skipped; nil is reported as an empty list, never null.
func (r ImportRun) ToResponse(rejections []dto.Rejection, ignored []string) dto.ImportResponse {
	if ignored == nil {
		ignored = []string{}
	}
	if rejections == nil {
		rejections = []dto.Rejection{}
	}
	return dto.ImportResponse{
		ImportID:       r.ID.String(),
		TotalRows:      r.TotalRows,
		ImportedRows:   r.ImportedRows,
		RejectedRows:   r.RejectedRows,
		Committed:      r.Committed,
		Rejections:     rejections,
		IgnoredColumns: ignored,
	}
}

// ToSummary is the run's row in the import activity list.
func (r ImportRun) ToSummary() dto.ImportSummary {
	return dto.ImportSummary{
		ImportID:     r.ID.String(),
		Filename:     r.Filename,
		ClientIP:     r.ClientIP,
		TotalRows:    r.TotalRows,
		ImportedRows: r.ImportedRows,
		RejectedRows: r.RejectedRows,
		Committed:    r.Committed,
		CreatedAt:    r.CreatedAt.UTC().Format(time.RFC3339),
	}
}

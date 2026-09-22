package model

import (
	"time"

	"github.com/google/uuid"
)

// ImportRun mirrors the import_runs table — one row per CSV upload.
type ImportRun struct {
	ID           uuid.UUID
	Filename     string
	ClientIP     string // caller IP (see ARCHITECTURE.md section 2); audit-only, not a security control
	TotalRows    int
	ImportedRows int
	RejectedRows int
	Committed    bool
	CreatedAt    time.Time `gorm:"<-:false"`
	UpdatedAt    time.Time `gorm:"<-:false"`
}

// TableName is the table ImportRun mirrors.
func (ImportRun) TableName() string { return "import_runs" }

// ImportRejection mirrors the import_rejections table — one row per
// rejected CSV row on a given ImportRun.
type ImportRejection struct {
	ID           int64
	ImportRunID  uuid.UUID
	CSVRowNumber int
	AssetID      *string
	Reason       string
	CreatedAt    time.Time `gorm:"<-:false"`
	UpdatedAt    time.Time `gorm:"<-:false"`
}

// TableName is the table ImportRejection mirrors.
func (ImportRejection) TableName() string { return "import_rejections" }

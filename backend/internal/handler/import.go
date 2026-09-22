package handler

import (
	"context"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"

	"smart-grid-asset-management/backend/internal/base/csvx"
	"smart-grid-asset-management/backend/internal/base/errx"
	"smart-grid-asset-management/backend/internal/base/httpx"
	"smart-grid-asset-management/backend/internal/defined"
	"smart-grid-asset-management/backend/internal/entity/dto"
	"smart-grid-asset-management/backend/internal/service"
)

type importService interface {
	Preview(ctx context.Context, in service.ImportInput) (dto.ImportPreviewResponse, error)
	Import(ctx context.Context, in service.ImportInput) (dto.ImportResponse, error)
	GetImport(ctx context.Context, req dto.ImportIDRequest) (dto.ImportResponse, error)
	ListImports(ctx context.Context, req dto.ImportListRequest) (dto.ImportListResponse, error)
	Template() []byte
}

// ImportHandler serves the CSV import endpoints.
type ImportHandler struct {
	service   importService
	maxMemory int64 // multipart parts above this spill to a temp file
}

func NewImportHandler(s importService, maxMemory int64) *ImportHandler {
	return &ImportHandler{service: s, maxMemory: maxMemory}
}

// readUpload extracts the CSV from the multipart request, checks its
// structure, and packages it with the request facts an import is audited with.
func (h *ImportHandler) readUpload(c *gin.Context) (service.ImportInput, error) {
	file, header, err := csvx.Extract(c.Request, defined.UploadField, h.maxMemory)
	if err != nil {
		return service.ImportInput{}, err
	}
	defer file.Close()

	parsed, err := csvx.Parse(file, header.Filename, csvx.Options{MinColumns: service.RequiredColumnCount()})
	if err != nil {
		return service.ImportInput{}, err
	}
	return service.ImportInput{
		Filename:            sanitizeFilename(header.Filename),
		ClientIP:            httpx.ClientIP(c.Request),
		CSV:                 parsed,
		ExpectedFingerprint: strings.TrimSpace(c.Request.FormValue(defined.FingerprintField)),
	}, nil
}

// withImportTimeout bounds the whole request like an import is bounded.
func withImportTimeout(c *gin.Context) context.CancelFunc {
	ctx, cancel := context.WithTimeout(c.Request.Context(), defined.ImportTimeout)
	c.Request = c.Request.WithContext(ctx)
	return cancel
}

// Preview godoc
// @Summary      Check a CSV file without importing it
// @Description  Validates the file exactly as `POST /api/imports` would and reports how many rows can be imported and which are rejected and why, **without storing anything** (no assets, no import record). Show this to the user, then confirm by sending the same file to `POST /api/imports` with `expected_fingerprint` set to the `fingerprint` returned here.
// @Description
// @Description  Column rules, date formats (`YYYY-MM-DD`, `D/M/YYYY`, `D/M/YY`, day first) and structural errors are the same as for the import.
// @Tags         imports
// @Accept       mpfd
// @Produce      json
// @Param        file  formData  file  true  "CSV file with a header row"
// @Success      200  {object}  dto.ImportPreviewResponse  "Validation outcome; nothing was stored"
// @Failure      400  {object}  dto.ErrorResponse   "No file supplied, or the file is empty / has no data rows"
// @Failure      413  {object}  dto.ErrorResponse   "File larger than the configured upload limit"
// @Failure      415  {object}  dto.ErrorResponse   "File does not have a .csv extension"
// @Failure      422  {object}  dto.ErrorResponse   "Malformed CSV (see `line`), or the header lacks a required column"
// @Failure      429  {object}  dto.ErrorResponse   "Rate limit exceeded"
// @Failure      500  {object}  dto.ErrorResponse   "Unexpected error; quote `trace_id` when reporting"
// @Failure      401  {object}  dto.ErrorResponse  "Missing or invalid x-api-key"
// @Security     ApiKeyAuth
// @Router       /api/imports/preview [post]
func (h *ImportHandler) Preview(c *gin.Context) {
	defer withImportTimeout(c)()
	in, err := h.readUpload(c)
	if err != nil {
		writeError(c, err)
		return
	}
	resp, err := h.service.Preview(c.Request.Context(), in)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

// Create godoc
// @Summary      Import assets from a CSV file
// @Description  Uploads a CSV of grid assets, validates every row again, and stores the rows that pass in one transaction. Rejected rows, and any row beneath a rejected parent, are skipped and reported; rows whose `asset_id` already exists are rejected, never updated. `committed` is true when at least one row was stored.
// @Description
// @Description  To let a user review the outcome first, call `POST /api/imports/preview` and send its `fingerprint` here as `expected_fingerprint`: if the file or the stored data changed since, nothing is written and the response is **412**. Without `expected_fingerprint` the rows that pass are stored directly.
// @Description
// @Description  **Columns are matched by header name**, in any order (case, spaces, `-` and `_` are ignored: `Asset ID` = `asset_id`). Required: `asset_id`, `asset_type`, `asset_name`, `operational_status`. Optional (may be absent from the file or blank): `parent_asset_id`, `voltage_kv`, `rating_kva`, `manufacturer`, `model`, `serial_number`, `commissioned_date` (`YYYY-MM-DD`, `D/M/YYYY` or `D/M/YY`, day first). Unknown columns are ignored and listed in `ignored_columns`. Allowed asset types, statuses and parent/child pairings are read from the database — see `GET /api/lookups`.
// @Description
// @Description  A file that is readable but has bad rows returns **200** with `rejections[]`; `csv_row` is the row's line in the uploaded file (header = line 1). A file that cannot be processed at all (not CSV, empty, required column missing) returns a 4xx `ErrorResponse` instead.
// @Tags         imports
// @Accept       mpfd
// @Produce      json
// @Param        file                 formData  file    true   "CSV file with a header row"
// @Param        expected_fingerprint formData  string  false  "Fingerprint from POST /api/imports/preview for this file"
// @Success      200  {object}  dto.ImportResponse  "Import outcome — check `committed`, `imported_rows` and `rejections`"
// @Failure      400  {object}  dto.ErrorResponse   "No file supplied, or the file is empty / has no data rows"
// @Failure      409  {object}  dto.ErrorResponse   "Another writer changed the stored data during the commit; nothing was stored, retry"
// @Failure      412  {object}  dto.ErrorResponse   "expected_fingerprint no longer matches; nothing was stored, preview again"
// @Failure      413  {object}  dto.ErrorResponse   "File larger than the configured upload limit"
// @Failure      415  {object}  dto.ErrorResponse   "File does not have a .csv extension"
// @Failure      422  {object}  dto.ErrorResponse   "Malformed CSV (see `line`), or the header lacks a required column"
// @Failure      429  {object}  dto.ErrorResponse   "Rate limit exceeded"
// @Failure      500  {object}  dto.ErrorResponse   "Unexpected error; quote `trace_id` when reporting"
// @Failure      401  {object}  dto.ErrorResponse  "Missing or invalid x-api-key"
// @Security     ApiKeyAuth
// @Router       /api/imports [post]
func (h *ImportHandler) Create(c *gin.Context) {
	defer withImportTimeout(c)()
	in, err := h.readUpload(c)
	if err != nil {
		writeError(c, err)
		return
	}
	resp, err := h.service.Import(c.Request.Context(), in)
	if err != nil {
		writeError(c, err)
		return
	}
	c.Header("Location", "/api/imports/"+resp.ImportID)
	c.JSON(http.StatusOK, resp)
}

// List godoc
// @Summary      List past imports
// @Description  One page of import activity, newest first: file name, client IP, row counts and whether data was committed. Imports only — deletions are not listed. Fetch `GET /api/imports/{importId}` for a run's rejected rows.
// @Tags         imports
// @Produce      json
// @Param        limit   query     int  false  "Page size"  default(25) minimum(1) maximum(100)
// @Param        offset  query     int  false  "Rows to skip"  default(0) minimum(0)
// @Success      200  {object}  dto.ImportListResponse
// @Failure      400  {object}  dto.ErrorResponse  "Bad limit or offset"
// @Failure      500  {object}  dto.ErrorResponse
// @Failure      401  {object}  dto.ErrorResponse  "Missing or invalid x-api-key"
// @Security     ApiKeyAuth
// @Router       /api/imports [get]
func (h *ImportHandler) List(c *gin.Context) {
	var req dto.ImportListRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		writeError(c, errx.InvalidInput("limit and offset must be integers"))
		return
	}
	resp, err := h.service.ListImports(c.Request.Context(), req)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

// Template godoc
// @Summary      Download the import template
// @Description  A CSV with every importable column in its header and a few fictional example rows (a small valid hierarchy) showing how parents, dates and statuses are written. Delete the example rows before importing your own data.
// @Tags         imports
// @Produce      text/csv
// @Success      200  {file}  file  "asset_import_template.csv"
// @Failure      401  {object}  dto.ErrorResponse  "Missing or invalid x-api-key"
// @Security     ApiKeyAuth
// @Router       /api/imports/template [get]
func (h *ImportHandler) Template(c *gin.Context) {
	c.Header("Content-Disposition", `attachment; filename="asset_import_template.csv"`)
	c.Data(http.StatusOK, "text/csv; charset=utf-8", h.service.Template())
}

// Get godoc
// @Summary      Get a past import
// @Description  Re-fetches the result of an earlier import, including every rejected row. `ignored_columns` is only reported on the upload response and is empty here.
// @Tags         imports
// @Produce      json
// @Param        importId  path      string  true  "Import id (UUID) from POST /api/imports"  format(uuid)
// @Success      200  {object}  dto.ImportResponse
// @Failure      400  {object}  dto.ErrorResponse  "importId is not a UUID"
// @Failure      404  {object}  dto.ErrorResponse  "No such import"
// @Failure      500  {object}  dto.ErrorResponse
// @Failure      401  {object}  dto.ErrorResponse  "Missing or invalid x-api-key"
// @Security     ApiKeyAuth
// @Router       /api/imports/{importId} [get]
func (h *ImportHandler) Get(c *gin.Context) {
	var req dto.ImportIDRequest
	if err := c.ShouldBindUri(&req); err != nil {
		writeError(c, errx.InvalidInput("invalid import id"))
		return
	}
	resp, err := h.service.GetImport(c.Request.Context(), req)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

// sanitizeFilename keeps only the base name, replaces control characters,
// and bounds the length: the name is stored and logged.
func sanitizeFilename(name string) string {
	name = filepath.Base(strings.ReplaceAll(name, `\`, "/"))
	name = strings.Map(func(r rune) rune {
		if r < ' ' || r == 0x7f {
			return '_'
		}
		return r
	}, name)
	if runes := []rune(name); len(runes) > defined.MaxFilenameRunes {
		name = string(runes[:defined.MaxFilenameRunes])
	}
	return name
}

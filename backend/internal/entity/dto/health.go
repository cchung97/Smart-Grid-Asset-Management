package dto

// HealthResponse is the response body for GET /healthz.
type HealthResponse struct {
	Status string `json:"status" validate:"required"`
	DB     string `json:"db" validate:"required"`
}

// Package router wires HTTP paths to handlers in one place, so the set
// of endpoints (and what guards them) can be read at a glance instead of
// spread across main.go.
package router

import (
	"net/http"

	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"

	"smart-grid-asset-management/backend/internal/entity/dto"
	"smart-grid-asset-management/backend/internal/handler"
	"smart-grid-asset-management/backend/internal/middleware"
)

// Deps are the handlers and shared middleware the router wires together.
type Deps struct {
	Health      *handler.HealthHandler
	Imports     *handler.ImportHandler
	Assets      *handler.AssetHandler
	Lookups     *handler.LookupHandler
	APIKey      string // guards every /api endpoint; /healthz and the docs are open
	CORSOrigins []string
	RateLimiter *middleware.RateLimiter
	// MaxRequestBytes caps every request body, and so the largest CSV upload
	// (see middleware.RequestSizeLimit).
	MaxRequestBytes int64
}

// New builds the engine: the global middleware chain plus route registration.
func New(deps Deps) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	e := gin.New()

	// Client IPs come from httpx.ClientIP, never from gin's proxy logic.
	_ = e.SetTrustedProxies(nil)

	// Exact paths only, and a 405 (with Allow) for the wrong verb.
	e.RedirectTrailingSlash = false
	e.RedirectFixedPath = false
	e.HandleMethodNotAllowed = true
	e.NoRoute(func(c *gin.Context) { c.JSON(http.StatusNotFound, dto.ErrorResponse{Error: "not found"}) })
	e.NoMethod(func(c *gin.Context) {
		c.JSON(http.StatusMethodNotAllowed, dto.ErrorResponse{Error: "method not allowed"})
	})

	// Order matters: TraceID first so everything downstream logs a trace ID;
	// AccessLog wraps Recover so a recovered panic is logged as the 500 it became.
	e.Use(
		middleware.TraceID,
		middleware.AccessLog,
		middleware.Recover,
		middleware.SecurityHeaders,
		middleware.CORS(deps.CORSOrigins),
		deps.RateLimiter.Middleware,
		middleware.RequestSizeLimit(deps.MaxRequestBytes),
	)

	e.GET("/healthz", deps.Health.Healthz)
	e.GET("/swagger/*any", middleware.DocsCSP, ginSwagger.WrapHandler(swaggerFiles.Handler))

	// Shortcut to the docs UI.
	e.GET("/docs", func(c *gin.Context) { c.Redirect(http.StatusFound, "/swagger/index.html") })

	// Business endpoints (docs/ARCHITECTURE.md section 3). Every one needs the
	// shared API key (x-api-key); only /healthz and the docs above are open. Where
	// a literal segment and a parameter overlap (roots/search/stats vs :assetId,
	// template vs :importId) the literal wins.
	api := e.Group("/api", middleware.RequireAPIKey(deps.APIKey))
	api.POST("/imports", deps.Imports.Create)
	api.POST("/imports/preview", deps.Imports.Preview)
	api.GET("/imports", deps.Imports.List)
	api.GET("/imports/template", deps.Imports.Template)
	api.GET("/imports/:importId", deps.Imports.Get)
	api.GET("/lookups", deps.Lookups.List)
	api.GET("/assets", deps.Assets.List)
	api.GET("/assets/roots", deps.Assets.Roots)
	api.GET("/assets/stats", deps.Assets.Stats)
	api.GET("/assets/search", deps.Assets.Search)
	api.GET("/assets/:assetId", deps.Assets.Get)
	api.DELETE("/assets/:assetId", deps.Assets.Delete)
	api.GET("/assets/:assetId/children", deps.Assets.Children)
	api.GET("/assets/:assetId/ancestors", deps.Assets.Ancestors)

	return e
}

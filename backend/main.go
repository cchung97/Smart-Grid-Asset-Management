// Package main is the entry point of the Smart Grid Asset Management backend.
//
// @title           Smart Grid Asset Management API
// @version         1.0
// @description     CSV asset import/validation and asset hierarchy explorer API.
//
// @BasePath  /
//
// @schemes http
//
// @securityDefinitions.apikey ApiKeyAuth
// @in header
// @name x-api-key
// @description API key (see API_KEY in .env). Required by every /api endpoint (sent as the x-api-key header). Only /healthz and these docs are open. This is a shared key, not user authentication (see ARCHITECTURE.md).
package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "smart-grid-asset-management/backend/docs"
	"smart-grid-asset-management/backend/internal/base/logs"
	"smart-grid-asset-management/backend/internal/base/postgres"
	"smart-grid-asset-management/backend/internal/config"
	"smart-grid-asset-management/backend/internal/defined"
	"smart-grid-asset-management/backend/internal/handler"
	"smart-grid-asset-management/backend/internal/middleware"
	"smart-grid-asset-management/backend/internal/repository"
	"smart-grid-asset-management/backend/internal/router"
	"smart-grid-asset-management/backend/internal/service"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		logs.WithCtx(context.Background()).Error("config error", "err", err)
		os.Exit(1)
	}

	appCtx, cancelApp := context.WithCancel(context.Background())
	defer cancelApp()

	if cfg.APIKey == "" {
		logs.WithCtx(appCtx).Warn("API_KEY is not set: every /api request will be refused")
	}

	db, err := postgres.New(appCtx, postgres.Config{
		DataSourceName: cfg.DB.DSN(),
	})
	if err != nil {
		logs.WithCtx(appCtx).Error("failed to configure database", "err", err)
		os.Exit(1)
	}
	defer postgres.Close(db)

	// Validation runs against an in-memory snapshot of the lookup tables;
	// load it before accepting traffic so a bad database fails startup, not
	// the first request.
	lookups := service.NewLookupCache(repository.NewLookupRepository(db))
	if err := lookups.Refresh(appCtx); err != nil {
		logs.WithCtx(appCtx).Error("failed to load lookup tables", "err", err)
		os.Exit(1)
	}

	rateLimiter := middleware.NewRateLimiter(appCtx, 0, 0) // defaults: 100 req/10s per client+path

	mux := router.New(router.Deps{
		Health:          handler.NewHealthHandler(service.NewHealthService(db)),
		Imports:         handler.NewImportHandler(service.NewImportService(db, lookups), min(cfg.MaxRequestBytes, defined.MaxUploadMemory)),
		Assets:          handler.NewAssetHandler(service.NewAssetService(repository.NewAssetRepository(db), lookups)),
		Lookups:         handler.NewLookupHandler(lookups),
		APIKey:          cfg.APIKey,
		CORSOrigins:     cfg.CORSOrigins,
		RateLimiter:     rateLimiter,
		MaxRequestBytes: cfg.MaxRequestBytes,
	})

	// Timeouts guard against slow-loris clients holding connections open;
	// Read/Write leave room for a large CSV upload and its validation.
	srv := &http.Server{
		Addr:              ":" + cfg.BackendPort,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       60 * time.Second,
		WriteTimeout:      3 * time.Minute, // > defined.ImportTimeout
		IdleTimeout:       120 * time.Second,
	}

	go func() {
		logs.WithCtx(appCtx).Info("listening", "port", cfg.BackendPort)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logs.WithCtx(appCtx).Error("server error", "err", err)
			os.Exit(1)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelShutdown()
	logs.WithCtx(appCtx).Info("shutting down")
	_ = srv.Shutdown(shutdownCtx)
}

package main

import (
	"database/sql"
	"fmt"
	"net/http"

	appexports "github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/exports"
	apprecovery "github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/recovery"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/db"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/identity"
	platformrecovery "github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/recovery"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/transport/http/handlers"
)

type recoveryExportRouteConfig struct {
	Database           *sql.DB
	RecoveryRoot       string
	Identity           identity.Provider
	Authenticator      *identity.RequestAuthenticator
	RateLimiter        handlers.RateLimiter
	AuditRecorder      handlers.AuditRecorder
	RateLimitPerMinute int
}

func registerRecoveryExportRoutes(router *http.ServeMux, config recoveryExportRouteConfig) error {
	if router == nil {
		return fmt.Errorf("recovery/export router is required")
	}
	repository, err := db.NewExportRepository(config.Database)
	if err != nil {
		return fmt.Errorf("initialize export repository: %w", err)
	}
	exportService, err := appexports.NewService(repository, appexports.DefaultMaxRows, appexports.DefaultPageSize)
	if err != nil {
		return fmt.Errorf("initialize export service: %w", err)
	}
	index, err := platformrecovery.NewManifestIndex(config.RecoveryRoot)
	if err != nil {
		return fmt.Errorf("initialize recovery evidence index: %w", err)
	}
	manifestService, err := apprecovery.NewManifestService(index)
	if err != nil {
		return fmt.Errorf("initialize recovery manifest service: %w", err)
	}
	handler := handlers.NewRecoveryExportHandler(manifestService, exportService, config.Identity).
		WithRateLimiter(config.RateLimiter, config.RateLimitPerMinute).
		WithAuditRecorder(config.AuditRecorder)
	if config.Authenticator != nil {
		handler.WithRequestAuthenticator(config.Authenticator)
	}
	router.HandleFunc("GET /api/recovery/manifests", handler.Manifests)
	router.HandleFunc("GET /api/exports/transfers.csv", handler.TransfersCSV)
	router.HandleFunc("GET /api/exports/accounts/{accountID}/transactions.csv", handler.AccountLedgerCSV)
	router.HandleFunc("GET /api/exports/reconciliation.csv", handler.ReconciliationCSV)
	return nil
}

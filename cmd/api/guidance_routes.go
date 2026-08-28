package main

import (
	"database/sql"
	"fmt"
	"net/http"

	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/guidance"
	apprecovery "github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/recovery"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/db"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/identity"
	platformrecovery "github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/recovery"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/transport/http/handlers"
)

type guidanceRouteConfig struct {
	Database           *sql.DB
	RecoveryRoot       string
	Identity           identity.Provider
	Authenticator      *identity.RequestAuthenticator
	RateLimiter        handlers.RateLimiter
	AuditRecorder      handlers.AuditRecorder
	RateLimitPerMinute int
}

func registerGuidanceRoutes(router *http.ServeMux, config guidanceRouteConfig) error {
	if router == nil {
		return fmt.Errorf("guidance router is required")
	}
	repository, err := db.NewGuidanceRepository(config.Database)
	if err != nil {
		return fmt.Errorf("initialize guidance repository: %w", err)
	}
	index, err := platformrecovery.NewManifestIndex(config.RecoveryRoot)
	if err != nil {
		return fmt.Errorf("initialize guidance recovery index: %w", err)
	}
	recoveryService, err := apprecovery.NewManifestService(index)
	if err != nil {
		return fmt.Errorf("initialize guidance recovery service: %w", err)
	}
	service, err := guidance.NewService(repository, recoveryService, nil)
	if err != nil {
		return fmt.Errorf("initialize guidance service: %w", err)
	}
	handler := handlers.NewGuidanceHandler(service, config.Identity).
		WithRateLimiter(config.RateLimiter, config.RateLimitPerMinute).
		WithAuditRecorder(config.AuditRecorder)
	if config.Authenticator != nil {
		handler.WithRequestAuthenticator(config.Authenticator)
	}
	router.HandleFunc("GET /api/local/orientation", handler.Orientation)
	router.HandleFunc("PUT /api/local/orientation/preferences", handler.UpdateOrientationPreferences)
	router.HandleFunc("GET /api/transfers/{transferID}/explainability", handler.ExplainTransfer)
	return nil
}

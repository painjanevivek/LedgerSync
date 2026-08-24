package main

import (
	"database/sql"
	"fmt"
	"net/http"

	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/reconciliation"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/db"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/identity"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/transport/http/handlers"
)

type reconciliationCommandRouteConfig struct {
	Database               *sql.DB
	Identity               identity.Provider
	Authenticator          *identity.RequestAuthenticator
	RateLimiter            handlers.RateLimiter
	AuditRecorder          handlers.AuditRecorder
	RateLimitPerMinute     int
	CapacityLimitPerSecond int
}

func registerReconciliationCommandRoutes(router *http.ServeMux, config reconciliationCommandRouteConfig) error {
	if router == nil {
		return fmt.Errorf("reconciliation command router is required")
	}
	repository, err := db.NewReconciliationRepository(config.Database)
	if err != nil {
		return fmt.Errorf("initialize reconciliation command repository: %w", err)
	}
	service, err := reconciliation.NewCommandService(repository, nil)
	if err != nil {
		return fmt.Errorf("initialize reconciliation command service: %w", err)
	}
	handler := handlers.NewReconciliationCommandHandler(service, config.Identity).
		WithRateLimiter(config.RateLimiter, config.RateLimitPerMinute).
		WithCapacityLimit(config.RateLimiter, config.CapacityLimitPerSecond).
		WithAuditRecorder(config.AuditRecorder)
	if config.Authenticator != nil {
		handler.WithRequestAuthenticator(config.Authenticator)
	}
	router.HandleFunc("POST /api/reconciliation/runs", handler.Run)
	return nil
}

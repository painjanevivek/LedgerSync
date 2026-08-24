package main

import (
	"database/sql"
	"fmt"
	"net/http"

	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/accounts"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/db"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/identity"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/transport/http/handlers"
)

type accountCommandRouteConfig struct {
	Database               *sql.DB
	Identity               identity.Provider
	Authenticator          *identity.RequestAuthenticator
	RateLimiter            handlers.RateLimiter
	AuditRecorder          handlers.AuditRecorder
	RateLimitPerMinute     int
	CapacityLimitPerSecond int
}

func registerAccountCommandRoutes(router *http.ServeMux, config accountCommandRouteConfig) error {
	if router == nil {
		return fmt.Errorf("account command router is required")
	}
	repository, err := db.NewAccountCommandRepository(config.Database, nil)
	if err != nil {
		return fmt.Errorf("initialize account command repository: %w", err)
	}
	service, err := accounts.NewCommandService(repository, nil)
	if err != nil {
		return fmt.Errorf("initialize account command service: %w", err)
	}
	handler := handlers.NewAccountCommandHandler(service, config.Identity).
		WithRateLimiter(config.RateLimiter, config.RateLimitPerMinute).
		WithCapacityLimit(config.RateLimiter, config.CapacityLimitPerSecond).
		WithAuditRecorder(config.AuditRecorder)
	if config.Authenticator != nil {
		handler.WithRequestAuthenticator(config.Authenticator)
	}
	router.HandleFunc("POST /api/accounts", handler.Create)
	router.HandleFunc("PATCH /api/accounts/{accountID}", handler.Patch)
	return nil
}

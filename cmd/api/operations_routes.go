package main

import (
	"database/sql"
	"fmt"
	"net/http"

	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/operations"
	cacheplatform "github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/cache"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/db"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/identity"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/transport/http/handlers"
	"github.com/redis/go-redis/v9"
)

type operationsRouteConfig struct {
	Database           *sql.DB
	Redis              *redis.Client
	Environment        string
	Identity           identity.Provider
	Authenticator      *identity.RequestAuthenticator
	RateLimiter        handlers.RateLimiter
	AuditRecorder      handlers.AuditRecorder
	RateLimitPerMinute int
}

func registerOperationsRoutes(router *http.ServeMux, config operationsRouteConfig) error {
	if router == nil {
		return fmt.Errorf("operations router is required")
	}
	repository, err := db.NewOperationsRepository(config.Database)
	if err != nil {
		return fmt.Errorf("initialize operations repository: %w", err)
	}
	cacheProbe, err := cacheplatform.NewHealthProbe(config.Redis)
	if err != nil {
		return fmt.Errorf("initialize cache health probe: %w", err)
	}
	diagnostics, err := operations.NewDiagnosticService(repository, cacheProbe, operations.CurrentBuildFacts(config.Environment), nil)
	if err != nil {
		return fmt.Errorf("initialize diagnostic service: %w", err)
	}
	events, err := operations.NewEventService(repository)
	if err != nil {
		return fmt.Errorf("initialize event evidence service: %w", err)
	}
	webhooks, err := operations.NewWebhookEndpointService(repository)
	if err != nil {
		return fmt.Errorf("initialize webhook endpoint evidence service: %w", err)
	}
	handler := handlers.NewOperationsHandler(diagnostics, events, webhooks, config.Identity).
		WithRateLimiter(config.RateLimiter, config.RateLimitPerMinute).
		WithAuditRecorder(config.AuditRecorder)
	if config.Authenticator != nil {
		handler.WithRequestAuthenticator(config.Authenticator)
	}
	router.HandleFunc("GET /api/local/diagnostics", handler.Diagnostics)
	router.HandleFunc("GET /api/events", handler.Events)
	router.HandleFunc("GET /api/events/{eventID}", handler.Event)
	router.HandleFunc("GET /api/webhook-endpoints", handler.WebhookEndpoints)
	router.HandleFunc("GET /api/webhook-endpoints/{endpointId}", handler.WebhookEndpoint)
	return nil
}

package main

import (
	"database/sql"
	"fmt"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/db"
	"net/http"

	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/identity"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/transport/http/handlers"
)

type developerRouteConfig struct {
	Database           *sql.DB
	StudyEnabled       bool
	Identity           identity.Provider
	Authenticator      *identity.RequestAuthenticator
	RateLimiter        handlers.RateLimiter
	AuditRecorder      handlers.AuditRecorder
	RateLimitPerMinute int
}

func registerDeveloperRoutes(router *http.ServeMux, config developerRouteConfig) error {
	if router == nil {
		return fmt.Errorf("developer contract router is required")
	}
	handler := handlers.NewDeveloperContractHandler(config.Identity).
		WithRateLimiter(config.RateLimiter, config.RateLimitPerMinute).
		WithAuditRecorder(config.AuditRecorder)
	if config.Authenticator != nil {
		handler.WithRequestAuthenticator(config.Authenticator)
	}
	router.HandleFunc("GET /api/developer/metadata", handler.Metadata)
	if config.StudyEnabled {
		handler.WithDistributedReader(db.NewDistributedEvidenceRepository(config.Database, true))
		router.HandleFunc("GET /api/developer/distributed", handler.Distributed)
	}
	router.HandleFunc("GET /api/openapi.yaml", handler.OpenAPI)
	return nil
}

package main

import (
	"fmt"
	statement "github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/statementreconciliation"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/db"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/transport/http/handlers"
	"net/http"
)

func registerStatementReconciliationRoutes(router *http.ServeMux, config reconciliationCommandRouteConfig) error {
	if router == nil {
		return fmt.Errorf("statement router is required")
	}
	repository, err := db.NewStatementReconciliationRepository(config.Database)
	if err != nil {
		return err
	}
	service, err := statement.NewService(repository)
	if err != nil {
		return err
	}
	handler := handlers.NewStatementReconciliationHandler(service, config.Identity).WithRequestAuthenticator(config.Authenticator).WithRateLimiter(config.RateLimiter, config.RateLimitPerMinute, config.RateLimitPerMinute, config.CapacityLimitPerSecond).WithAuditRecorder(config.AuditRecorder)
	router.HandleFunc("GET /api/statement-reconciliation", handler.Workspace)
	router.HandleFunc("POST /api/statement-reconciliation/mappings", handler.CreateMapping)
	router.HandleFunc("POST /api/statement-reconciliation/preview", handler.Preview)
	router.HandleFunc("POST /api/statement-reconciliation/imports", handler.Import)
	router.HandleFunc("POST /api/statement-reconciliation/runs", handler.Run)
	router.HandleFunc("GET /api/statement-reconciliation/runs/{runID}", handler.RunDetail)
	router.HandleFunc("POST /api/statement-reconciliation/runs/{runID}/reviews", handler.Review)
	router.HandleFunc("GET /api/statement-reconciliation/imports/{importID}/source.csv", handler.Source)
	router.HandleFunc("GET /api/statement-reconciliation/runs/{runID}/comparison.csv", handler.Export)
	return nil
}

package main

import (
	"database/sql"
	"errors"
	"net/http"

	appapprovals "github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/approvals"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/db"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/identity"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/transport/http/handlers"
)

type approvalRouteConfig struct {
	Database          *sql.DB
	Identity          identity.Provider
	Authenticator     *identity.RequestAuthenticator
	RateLimiter       handlers.RateLimiter
	ReadRatePerMinute int
}

func registerApprovalRoutes(router *http.ServeMux, config approvalRouteConfig) error {
	if router == nil || config.Database == nil || config.Identity == nil {
		return errors.New("approval route dependencies are required")
	}
	repository, err := db.NewApprovalRepository(config.Database)
	if err != nil {
		return err
	}
	service, err := appapprovals.NewService(repository, nil)
	if err != nil {
		return err
	}
	handler := handlers.NewApprovalHandler(service, config.Identity).
		WithRequestAuthenticator(config.Authenticator).
		WithRateLimiter(config.RateLimiter, config.ReadRatePerMinute)
	router.HandleFunc("GET /api/approvals", handler.List)
	return nil
}

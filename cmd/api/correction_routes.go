package main

import (
	"database/sql"
	"errors"
	"net/http"

	appcorrections "github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/corrections"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/db"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/identity"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/transport/http/handlers"
)

type correctionRouteConfig struct {
	Database               *sql.DB
	Identity               identity.Provider
	Authenticator          *identity.RequestAuthenticator
	RateLimiter            handlers.RateLimiter
	AuditRecorder          handlers.AuditRecorder
	ReadRatePerMinute      int
	WriteRatePerMinute     int
	CapacityLimitPerSecond int
}

func registerCorrectionRoutes(router *http.ServeMux, config correctionRouteConfig) error {
	if router == nil || config.Database == nil || config.Identity == nil {
		return errors.New("correction route dependencies are required")
	}
	repository, err := db.NewTransferCorrectionRepository(config.Database, nil)
	if err != nil {
		return err
	}
	service, err := appcorrections.NewService(repository, nil)
	if err != nil {
		return err
	}
	handler := handlers.NewCorrectionHandler(service, config.Identity).
		WithRequestAuthenticator(config.Authenticator).
		WithRateLimiter(config.RateLimiter, config.ReadRatePerMinute, config.WriteRatePerMinute, config.CapacityLimitPerSecond).
		WithAuditRecorder(config.AuditRecorder)

	router.HandleFunc("POST /api/transfers/{transferID}/corrections", handler.Request)
	router.HandleFunc("GET /api/transfer-corrections", handler.List)
	router.HandleFunc("GET /api/transfer-corrections/{correctionId}", handler.Get)
	router.HandleFunc("POST /api/transfer-corrections/{correctionId}/approve", handler.Approve)
	router.HandleFunc("POST /api/transfer-corrections/{correctionId}/reject", handler.Reject)
	router.HandleFunc("POST /api/transfer-corrections/{correctionId}/cancel", handler.Cancel)
	router.HandleFunc("POST /api/transfer-corrections/{correctionId}/post", handler.Post)
	return nil
}

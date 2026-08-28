package main

import (
	"database/sql"
	"errors"
	"net/http"

	developerplatform "github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/developerplatform"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/db"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/identity"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/transport/http/handlers"
)

type developerPlatformRouteConfig struct {
	Database               *sql.DB
	Environment            string
	Identity               identity.Provider
	Authenticator          *identity.RequestAuthenticator
	RateLimiter            handlers.RateLimiter
	AuditRecorder          handlers.AuditRecorder
	ReadRatePerMinute      int
	WriteRatePerMinute     int
	CapacityLimitPerSecond int
}

func registerDeveloperPlatformRoutes(router *http.ServeMux, config developerPlatformRouteConfig) error {
	if router == nil || config.Database == nil || config.Identity == nil {
		return errors.New("developer platform route dependencies are required")
	}
	repository, err := db.NewDeveloperCredentialRepository(config.Database, nil)
	if err != nil {
		return err
	}
	service, err := developerplatform.NewCredentialService(repository, nil)
	if err != nil {
		return err
	}
	handler := handlers.NewDeveloperCredentialHandler(service, config.Identity).
		WithRequestAuthenticator(config.Authenticator).
		WithRateLimiter(config.RateLimiter, config.ReadRatePerMinute, config.WriteRatePerMinute, config.CapacityLimitPerSecond).
		WithAuditRecorder(config.AuditRecorder).
		WithProductionStepUp(config.Environment != "development")

	router.HandleFunc("POST /api/developer/credentials", handler.Create)
	router.HandleFunc("GET /api/developer/credentials", handler.List)
	router.HandleFunc("GET /api/developer/credentials/{credentialId}", handler.Get)
	router.HandleFunc("POST /api/developer/credentials/{credentialId}/rotations", handler.Rotate)
	router.HandleFunc("POST /api/developer/credentials/{credentialId}/revocations", handler.Revoke)

	webhookRepository, err := db.NewDeveloperWebhookRepository(config.Database, nil)
	if err != nil {
		return err
	}
	deliveryReplayRepository, err := db.NewDeliveryReplayRepository(config.Database, nil)
	if err != nil {
		return err
	}
	webhookService, err := developerplatform.NewWebhookService(webhookRepository, deliveryReplayRepository, config.Environment, nil, nil)
	if err != nil {
		return err
	}
	webhookHandler := handlers.NewDeveloperWebhookHandler(webhookService, config.Identity).
		WithRequestAuthenticator(config.Authenticator).
		WithRateLimiter(config.RateLimiter, config.ReadRatePerMinute, config.WriteRatePerMinute, config.CapacityLimitPerSecond).
		WithAuditRecorder(config.AuditRecorder).
		WithProductionStepUp(config.Environment != "development")
	router.HandleFunc("POST /api/developer/webhooks", webhookHandler.Register)
	router.HandleFunc("GET /api/developer/webhooks", webhookHandler.List)
	router.HandleFunc("GET /api/developer/webhooks/{webhookId}", webhookHandler.Get)
	router.HandleFunc("POST /api/developer/webhooks/{webhookId}/verifications", webhookHandler.Verify)
	router.HandleFunc("POST /api/developer/webhooks/{webhookId}/signature-rotations", webhookHandler.Rotate)
	router.HandleFunc("POST /api/developer/webhooks/{webhookId}/disablements", webhookHandler.Disable)
	router.HandleFunc("GET /api/developer/webhooks/{webhookId}/deliveries", webhookHandler.Deliveries)
	router.HandleFunc("POST /api/developer/webhooks/{webhookId}/deliveries/{attemptId}/replay-approvals", webhookHandler.ApproveReplay)
	router.HandleFunc("POST /api/developer/webhooks/{webhookId}/deliveries/{attemptId}/replays", webhookHandler.Replay)
	return nil
}

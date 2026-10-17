package main

import (
	hold "github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/reservations"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/db"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/transport/http/handlers"
	"net/http"
)

func registerReservationRoutes(router *http.ServeMux, config reconciliationCommandRouteConfig) error {
	repository, err := db.NewReservationRepository(config.Database)
	if err != nil {
		return err
	}
	service, err := hold.NewService(repository)
	if err != nil {
		return err
	}
	handler := handlers.NewReservationHandler(service, config.Identity).WithRequestAuthenticator(config.Authenticator).WithRateLimiter(config.RateLimiter, config.RateLimitPerMinute, config.RateLimitPerMinute, config.CapacityLimitPerSecond).WithAuditRecorder(config.AuditRecorder)
	router.HandleFunc("GET /api/reservations", handler.List)
	router.HandleFunc("GET /api/reservations/export.csv", handler.Export)
	router.HandleFunc("POST /api/reservations", handler.Create)
	router.HandleFunc("GET /api/reservations/{holdID}", handler.Read)
	router.HandleFunc("POST /api/reservations/{holdID}/decisions", handler.Decide)
	return nil
}

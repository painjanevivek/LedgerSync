package main

import (
	"database/sql"
	"fmt"
	"net/http"

	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/db"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/identity"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/transport/http/handlers"
)

func registerSessionRoutes(router *http.ServeMux, database *sql.DB, provider identity.Provider) error {
	if router == nil {
		return fmt.Errorf("session router is required")
	}
	repository, err := db.NewSessionRepository(database)
	if err != nil {
		return fmt.Errorf("initialize session repository: %w", err)
	}
	handler := handlers.NewSessionHandler(repository, provider)
	router.HandleFunc("POST /api/internal/bff/sessions", handler.Create)
	router.HandleFunc("POST /api/internal/bff/sessions/resolve", handler.Resolve)
	router.HandleFunc("POST /api/internal/bff/sessions/revoke", handler.Revoke)
	router.HandleFunc("PATCH /api/internal/bff/sessions/consistency", handler.UpdateConsistency)
	return nil
}

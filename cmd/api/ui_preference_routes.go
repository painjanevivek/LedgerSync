package main

import (
	"database/sql"
	"fmt"
	"net/http"

	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/db"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/identity"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/transport/http/handlers"
)

func registerUIPreferenceRoutes(router *http.ServeMux, database *sql.DB, provider identity.Provider, authenticator *identity.RequestAuthenticator) error {
	if router == nil {
		return fmt.Errorf("UI preference router is required")
	}
	repository, err := db.NewUIPreferenceRepository(database)
	if err != nil {
		return fmt.Errorf("initialize UI preference repository: %w", err)
	}
	handler := handlers.NewUIPreferenceHandler(repository, provider, authenticator)
	router.HandleFunc("GET /api/ui/preferences", handler.Get)
	router.HandleFunc("PATCH /api/ui/preferences", handler.Patch)
	return nil
}

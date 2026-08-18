package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/config"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/observability"
	httptransport "github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/transport/http"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/transport/http/middleware"
)

func main() {
	slog.SetDefault(observability.NewLogger(slog.NewJSONHandler(os.Stderr, nil)))
	configuration, err := config.Load()
	if err != nil {
		slog.Error("configuration invalid", "error", err)
		os.Exit(1)
	}

	handler := middleware.Correlation(httptransport.NewHealthHandler(nil))
	server := &http.Server{Addr: configuration.HTTPAddress, Handler: handler}
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-stop
		context, cancel := context.WithTimeout(context.Background(), configuration.ShutdownTimeout)
		defer cancel()
		_ = server.Shutdown(context)
	}()

	slog.Info("LedgerSync API starting", "address", configuration.HTTPAddress, "environment", configuration.Environment)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		slog.Error("API stopped unexpectedly", "error", err)
		os.Exit(1)
	}
}

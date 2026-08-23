package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/accounts"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/consistency"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/transactions"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/transfers"
	cacheplatform "github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/cache"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/config"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/db"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/identity"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/observability"
	httptransport "github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/transport/http"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/transport/http/handlers"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/transport/http/middleware"
	"github.com/redis/go-redis/v9"
)

func main() {
	slog.SetDefault(observability.NewLogger(slog.NewJSONHandler(os.Stderr, nil)))
	configuration, err := config.Load()
	if err != nil {
		slog.Error("configuration invalid", "error", err)
		os.Exit(1)
	}
	telemetry, err := observability.NewTelemetry(context.Background(), observability.TelemetryConfig{Enabled: configuration.TelemetryEnabled, ServiceName: configuration.TelemetryServiceName, Endpoint: configuration.OTLPHTTPEndpoint})
	if err != nil {
		slog.Error("telemetry initialization failed", "error", err)
		os.Exit(1)
	}
	defer func() { _ = telemetry.Shutdown(context.Background()) }()

	var readiness httptransport.DependencyCheck
	router := http.NewServeMux()
	if configuration.DatabaseURL != "" {
		database, err := db.OpenPool(context.Background(), db.PoolConfig{DriverName: "pgx", DSN: configuration.DatabaseURL})
		if err != nil {
			slog.Error("database initialization failed", "error", err)
			os.Exit(1)
		}
		defer database.Close()
		readiness = database.Ping
		if configuration.Environment != "development" || (configuration.DevelopmentSubjectID != "" && configuration.DevelopmentTenantID != "") {
			if configuration.RedisAddress == "" || len(configuration.ConsistencySigningKey) < 32 {
				slog.Error("balance consistency configuration is missing")
				os.Exit(1)
			}
			redisClient := redis.NewClient(&redis.Options{Addr: configuration.RedisAddress})
			defer redisClient.Close()
			if err := redisClient.Ping(context.Background()).Err(); err != nil {
				slog.Error("redis initialization failed", "error", err)
				os.Exit(1)
			}
			issuer, err := consistency.NewIssuer(consistency.Key{ID: configuration.ConsistencySigningKeyID, Secret: []byte(configuration.ConsistencySigningKey)}, nil, nil, 10*time.Minute)
			if err != nil {
				slog.Error("consistency issuer initialization failed", "error", err)
				os.Exit(1)
			}
			balanceCache, err := cacheplatform.NewBalanceCache(redisClient, "", 5*time.Minute, telemetry)
			if err != nil {
				slog.Error("balance cache initialization failed", "error", err)
				os.Exit(1)
			}
			cacheAdapter, err := cacheplatform.NewAccountAdapter(balanceCache)
			if err != nil {
				slog.Error("balance cache adapter initialization failed", "error", err)
				os.Exit(1)
			}
			balanceRepository, err := db.NewBalanceRepository(database)
			if err != nil {
				slog.Error("balance repository initialization failed", "error", err)
				os.Exit(1)
			}
			ryewMetrics := observability.NewRYEWMetrics(telemetry)
			balanceReader, err := accounts.NewReader(balanceRepository, cacheAdapter, issuer, accounts.ReaderConfig{Metrics: ryewMetrics})
			if err != nil {
				slog.Error("balance reader initialization failed", "error", err)
				os.Exit(1)
			}
			repository, err := db.NewTransferRepository(database, nil, telemetry)
			if err != nil {
				slog.Error("transfer repository initialization failed", "error", err)
				os.Exit(1)
			}
			metrics := observability.NewTransferMetrics(telemetry)
			service, err := transfers.NewService(repository, nil, metrics)
			if err != nil {
				slog.Error("transfer service initialization failed", "error", err)
				os.Exit(1)
			}
			var provider identity.Provider
			if configuration.Environment == "development" {
				provider = identity.DevelopmentProvider{SubjectID: configuration.DevelopmentSubjectID, TenantID: configuration.DevelopmentTenantID, Scopes: []string{"accounts:read", "transactions:read", "transfers:write"}}
			} else {
				provider, err = identity.NewOIDCProvider(context.Background(), configuration.OIDCIssuerURL, configuration.OIDCAudience)
				if err != nil {
					slog.Error("OIDC provider initialization failed", "error", err)
					os.Exit(1)
				}
			}
			accountRepository, err := db.NewAccountRepository(database)
			if err != nil {
				slog.Error("account repository initialization failed", "error", err)
				os.Exit(1)
			}
			accountService, err := accounts.NewService(accountRepository)
			if err != nil {
				slog.Error("account service initialization failed", "error", err)
				os.Exit(1)
			}
			historyRepository, err := db.NewTransactionHistoryRepository(database)
			if err != nil {
				slog.Error("history repository initialization failed", "error", err)
				os.Exit(1)
			}
			history, err := transactions.NewHistory(historyRepository)
			if err != nil {
				slog.Error("history service initialization failed", "error", err)
				os.Exit(1)
			}
			transferHandler := handlers.NewTransferHandler(service, provider, issuer)
			balanceHandler := handlers.NewBalanceHandler(balanceReader, provider)
			accountsHandler := handlers.NewAccountsHandler(accountService, provider)
			transactionsHandler := handlers.NewTransactionsHandler(history, provider)
			investigationRepository, err := db.NewInvestigationRepository(database)
			if err != nil {
				slog.Error("investigation repository initialization failed", "error", err)
				os.Exit(1)
			}
			investigationHandler := handlers.NewInvestigationHandler(investigationRepository, provider)
			if len(configuration.BFFAssertionSecret) >= 32 {
				transferHandler.WithBFFAssertionSecret(configuration.BFFAssertionSecret)
				balanceHandler.WithBFFAssertionSecret(configuration.BFFAssertionSecret)
				accountsHandler.WithBFFAssertionSecret(configuration.BFFAssertionSecret)
				transactionsHandler.WithBFFAssertionSecret(configuration.BFFAssertionSecret)
				investigationHandler.WithBFFAssertionSecret(configuration.BFFAssertionSecret)
			}
			router.Handle("POST /api/transfers", transferHandler)
			router.Handle("GET /api/accounts/{accountID}/balance", balanceHandler)
			router.Handle("GET /api/me/accounts", accountsHandler)
			router.Handle("GET /api/accounts/{accountID}/transactions", transactionsHandler)
			router.HandleFunc("GET /api/transfers", investigationHandler.Transfers)
			router.HandleFunc("GET /api/transfers/{transferID}", investigationHandler.Transfer)
			router.HandleFunc("GET /api/reconciliation/runs", investigationHandler.ReconciliationRuns)
			router.HandleFunc("GET /api/reconciliation/runs/{runID}", investigationHandler.ReconciliationRun)
		}
	}
	router.Handle("/", httptransport.NewHealthHandler(readiness))
	handler := middleware.Correlation(telemetry.HTTP(router))
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

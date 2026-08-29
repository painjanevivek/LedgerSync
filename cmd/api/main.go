package main

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/accounts"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/consistency"
	appfunding "github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/funding"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/transactions"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/transfers"
	cacheplatform "github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/cache"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/config"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/db"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/identity"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/observability"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/startup"
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
	startupContext, stopStartup := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stopStartup()
	startupConfig := startup.Config{
		Timeout:        configuration.StartupTimeout,
		InitialBackoff: configuration.StartupInitialBackoff,
		MaxBackoff:     configuration.StartupMaxBackoff,
		OnRetry: func(event startup.Event) {
			slog.Warn("dependency not ready during bounded startup", "dependency", event.Dependency, "attempt", event.Attempt, "category", event.Category, "retry_in", event.Delay, "startup_time_remaining", event.Remaining, "error", event.Err)
		},
	}

	var readiness httptransport.DependencyCheck
	router := http.NewServeMux()
	if configuration.DatabaseURL != "" {
		database, err := startup.Open(startupContext, "postgresql", startupConfig, func(ctx context.Context) (*sql.DB, error) {
			return db.OpenPool(ctx, db.PoolConfig{DriverName: "pgx", DSN: configuration.DatabaseURL})
		})
		if err != nil {
			slog.Error("database initialization failed", "error", err)
			os.Exit(1)
		}
		defer func() {
			if closeErr := database.Close(); closeErr != nil {
				slog.Warn("database close failed", "error", closeErr)
			}
		}()
		readiness = database.Ping
		if configuration.Environment != "development" {
			if err := db.ValidatePilotCurrency(context.Background(), database, configuration.PilotCurrency); err != nil {
				slog.Error("pilot currency validation failed", "error", err)
				os.Exit(1)
			}
		}
		if configuration.Environment != "development" || (configuration.DevelopmentSubjectID != "" && configuration.DevelopmentTenantID != "") {
			if configuration.RedisAddress == "" || len(configuration.ConsistencySigningKey) < 32 {
				slog.Error("balance consistency configuration is missing")
				os.Exit(1)
			}
			redisClient, err := startup.Open(startupContext, "redis", startupConfig, func(ctx context.Context) (*redis.Client, error) {
				client := redis.NewClient(&redis.Options{Addr: configuration.RedisAddress})
				if pingErr := client.Ping(ctx).Err(); pingErr != nil {
					_ = client.Close()
					return nil, pingErr
				}
				return client, nil
			})
			if err != nil {
				slog.Error("redis initialization failed", "error", err)
				os.Exit(1)
			}
			defer func() {
				if closeErr := redisClient.Close(); closeErr != nil {
					slog.Warn("redis close failed", "error", closeErr)
				}
			}()
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
			repository.WithPilotCurrency(configuration.PilotCurrency)
			var provider identity.Provider
			if configuration.Environment == "development" {
				provider = identity.DevelopmentProvider{SubjectID: configuration.DevelopmentSubjectID, TenantID: configuration.DevelopmentTenantID, Credential: configuration.DevelopmentAPIToken, Roles: []string{"tenant:operator"}, Scopes: []string{"accounts:read", "accounts:write", "transactions:read", "transfers:read", "transfers:write", "reconciliation:read", "reconciliation:write", "local:read", "local:write", "events:read", "developer:read", "credentials:read", "credentials:write", "webhooks:read", "webhooks:write", "webhooks:replay", "recovery:read", "exports:read", "explainability:read", "funding:read", "funding:write", "funding:approve", "corrections:read", "corrections:write", "corrections:approve", identity.BFFActorScope}}
			} else {
				provider, err = identity.NewOIDCProvider(context.Background(), identity.OIDCProviderConfig{
					IssuerURL:        configuration.OIDCIssuerURL,
					ResourceAudience: configuration.OIDCResourceAudience,
					ClientTenants:    configuration.OIDCClientTenantMap,
				})
				if err != nil {
					slog.Error("OIDC provider initialization failed", "error", err)
					os.Exit(1)
				}
				credentialUsageRepository, usageErr := db.NewDeveloperCredentialRepository(database, nil)
				if usageErr != nil {
					slog.Error("credential usage repository initialization failed", "error", usageErr)
					os.Exit(1)
				}
				provider, usageErr = identity.NewUsageTrackingProvider(provider, credentialUsageRepository, nil)
				if usageErr != nil {
					slog.Error("credential usage tracking initialization failed", "error", usageErr)
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
			transferHandler := handlers.NewTransferHandler(service, provider, issuer).WithConsistencyBalanceReader(balanceRepository)
			balanceHandler := handlers.NewBalanceHandler(balanceReader, provider)
			accountsHandler := handlers.NewAccountsHandler(accountService, provider)
			transactionsHandler := handlers.NewTransactionsHandler(history, provider)
			fundingRepository, err := db.NewFundingRepository(database, nil)
			if err != nil {
				slog.Error("funding repository initialization failed", "error", err)
				os.Exit(1)
			}
			fundingPolicy := appfunding.PolicyProductionDualControl
			if configuration.Environment == "development" {
				fundingPolicy = appfunding.PolicyLocalDemoSingleOperator
			}
			fundingService, err := appfunding.NewService(fundingRepository, fundingPolicy, nil)
			if err != nil {
				slog.Error("funding service initialization failed", "error", err)
				os.Exit(1)
			}
			fundingHandler := handlers.NewFundingHandler(fundingService, provider)
			investigationRepository, err := db.NewInvestigationRepository(database)
			if err != nil {
				slog.Error("investigation repository initialization failed", "error", err)
				os.Exit(1)
			}
			investigationHandler := handlers.NewInvestigationHandler(investigationRepository, provider)
			rateLimiter, err := db.NewRateLimitRepository(database, nil)
			if err != nil {
				slog.Error("rate limiter initialization failed", "error", err)
				os.Exit(1)
			}
			transferHandler.WithRateLimiter(rateLimiter, configuration.WriteRateLimitPerMinute)
			transferHandler.WithCapacityLimit(rateLimiter, configuration.WriteCapacityPerSecond)
			balanceHandler.WithRateLimiter(rateLimiter, configuration.ReadRateLimitPerMinute)
			accountsHandler.WithRateLimiter(rateLimiter, configuration.ReadRateLimitPerMinute)
			transactionsHandler.WithRateLimiter(rateLimiter, configuration.ReadRateLimitPerMinute)
			investigationHandler.WithRateLimiter(rateLimiter, configuration.ReadRateLimitPerMinute)
			fundingHandler.WithRateLimiter(rateLimiter, configuration.ReadRateLimitPerMinute, configuration.WriteRateLimitPerMinute, configuration.WriteCapacityPerSecond)
			auditRepository, err := db.NewAuditRepository(database)
			if err != nil {
				slog.Error("audit repository initialization failed", "error", err)
				os.Exit(1)
			}
			transferHandler.WithAuditRecorder(auditRepository)
			balanceHandler.WithAuditRecorder(auditRepository)
			accountsHandler.WithAuditRecorder(auditRepository)
			transactionsHandler.WithAuditRecorder(auditRepository)
			investigationHandler.WithAuditRecorder(auditRepository)
			fundingHandler.WithAuditRecorder(auditRepository)
			var authenticator *identity.RequestAuthenticator
			if len(configuration.BFFAssertionSecret) >= 32 {
				replayGuard, guardErr := identity.NewPostgresReplayGuard(database, nil)
				if guardErr != nil {
					slog.Error("BFF actor assertion replay store initialization failed", "error", guardErr)
					os.Exit(1)
				}
				assertionConfig := identity.ActorAssertionConfig{Issuer: configuration.BFFAssertionIssuer, Audience: configuration.BFFAssertionAudience, CurrentKey: identity.ActorAssertionKey{ID: configuration.BFFAssertionKeyID, Secret: []byte(configuration.BFFAssertionSecret)}, MaxLifetime: time.Minute, ClockSkew: 5 * time.Second, ReplayGuard: replayGuard}
				if configuration.BFFAssertionPreviousSecret != "" {
					assertionConfig.PreviousKey = &identity.ActorAssertionKey{ID: configuration.BFFAssertionPreviousKeyID, Secret: []byte(configuration.BFFAssertionPreviousSecret)}
				}
				authenticator, err = identity.NewRequestAuthenticatorWithConfig(provider, assertionConfig)
				if err != nil {
					slog.Error("BFF actor assertion configuration is invalid")
					os.Exit(1)
				}
				transferHandler.WithRequestAuthenticator(authenticator)
				balanceHandler.WithRequestAuthenticator(authenticator)
				accountsHandler.WithRequestAuthenticator(authenticator)
				transactionsHandler.WithRequestAuthenticator(authenticator)
				investigationHandler.WithRequestAuthenticator(authenticator)
				fundingHandler.WithRequestAuthenticator(authenticator)
			}
			if err := registerAccountCommandRoutes(router, accountCommandRouteConfig{
				Database: database, Identity: provider, Authenticator: authenticator, RateLimiter: rateLimiter, AuditRecorder: auditRepository,
				RateLimitPerMinute: configuration.WriteRateLimitPerMinute, CapacityLimitPerSecond: configuration.WriteCapacityPerSecond,
			}); err != nil {
				slog.Error("account command route initialization failed", "error", err)
				os.Exit(1)
			}
			if err := registerReconciliationCommandRoutes(router, reconciliationCommandRouteConfig{
				Database: database, Identity: provider, Authenticator: authenticator, RateLimiter: rateLimiter, AuditRecorder: auditRepository,
				RateLimitPerMinute: configuration.WriteRateLimitPerMinute, CapacityLimitPerSecond: configuration.WriteCapacityPerSecond,
			}); err != nil {
				slog.Error("reconciliation command route initialization failed", "error", err)
				os.Exit(1)
			}
			if err := registerOperationsRoutes(router, operationsRouteConfig{
				Database: database, Redis: redisClient, Environment: configuration.Environment, Identity: provider, Authenticator: authenticator,
				RateLimiter: rateLimiter, AuditRecorder: auditRepository, RateLimitPerMinute: configuration.ReadRateLimitPerMinute,
			}); err != nil {
				slog.Error("operations route initialization failed", "error", err)
				os.Exit(1)
			}
			if err := registerDeveloperRoutes(router, developerRouteConfig{
				Identity: provider, Authenticator: authenticator, RateLimiter: rateLimiter, AuditRecorder: auditRepository,
				RateLimitPerMinute: configuration.ReadRateLimitPerMinute,
			}); err != nil {
				slog.Error("developer contract route initialization failed", "error", err)
				os.Exit(1)
			}
			if err := registerDeveloperPlatformRoutes(router, developerPlatformRouteConfig{
				Database: database, Environment: configuration.Environment, Identity: provider, Authenticator: authenticator,
				RateLimiter: rateLimiter, AuditRecorder: auditRepository, ReadRatePerMinute: configuration.ReadRateLimitPerMinute,
				WriteRatePerMinute: configuration.WriteRateLimitPerMinute, CapacityLimitPerSecond: configuration.WriteCapacityPerSecond,
			}); err != nil {
				slog.Error("developer platform route initialization failed", "error", err)
				os.Exit(1)
			}
			if err := registerRecoveryExportRoutes(router, recoveryExportRouteConfig{
				Database: database, RecoveryRoot: configuration.RecoveryEvidenceRoot, Identity: provider, Authenticator: authenticator,
				RateLimiter: rateLimiter, AuditRecorder: auditRepository, RateLimitPerMinute: configuration.ReadRateLimitPerMinute,
			}); err != nil {
				slog.Error("recovery/export route initialization failed", "error", err)
				os.Exit(1)
			}
			if err := registerGuidanceRoutes(router, guidanceRouteConfig{
				Database: database, RecoveryRoot: configuration.RecoveryEvidenceRoot, Identity: provider, Authenticator: authenticator,
				RateLimiter: rateLimiter, AuditRecorder: auditRepository, RateLimitPerMinute: configuration.ReadRateLimitPerMinute,
			}); err != nil {
				slog.Error("guidance route initialization failed", "error", err)
				os.Exit(1)
			}
			if err := registerCorrectionRoutes(router, correctionRouteConfig{
				Database: database, Identity: provider, Authenticator: authenticator, RateLimiter: rateLimiter, AuditRecorder: auditRepository,
				ReadRatePerMinute: configuration.ReadRateLimitPerMinute, WriteRatePerMinute: configuration.WriteRateLimitPerMinute,
				CapacityLimitPerSecond: configuration.WriteCapacityPerSecond,
			}); err != nil {
				slog.Error("correction route initialization failed", "error", err)
				os.Exit(1)
			}
			router.Handle("POST /api/transfers", transferHandler)
			router.Handle("GET /api/accounts/{accountID}/balance", balanceHandler)
			router.Handle("GET /api/me/accounts", accountsHandler)
			router.Handle("GET /api/accounts/{accountID}", accountsHandler)
			router.Handle("GET /api/accounts/{accountID}/transactions", transactionsHandler)
			router.HandleFunc("GET /api/transfers", investigationHandler.Transfers)
			router.HandleFunc("GET /api/transfers/{transferID}", investigationHandler.Transfer)
			router.HandleFunc("GET /api/reconciliation/runs", investigationHandler.ReconciliationRuns)
			router.HandleFunc("GET /api/reconciliation/runs/{runID}", investigationHandler.ReconciliationRun)
			router.HandleFunc("POST /api/funding-requests", fundingHandler.Request)
			router.HandleFunc("GET /api/funding-events", fundingHandler.List)
			router.HandleFunc("GET /api/funding-events/{fundingEventId}", fundingHandler.Get)
			router.HandleFunc("POST /api/funding-events/{fundingEventId}/approve", fundingHandler.Approve)
			router.HandleFunc("POST /api/funding-events/{fundingEventId}/reject", fundingHandler.Reject)
			router.HandleFunc("POST /api/funding-events/{fundingEventId}/post", fundingHandler.Post)
			router.HandleFunc("POST /api/funding-events/{fundingEventId}/compensations", fundingHandler.Compensate)
			router.HandleFunc("GET /api/funding-events/{fundingEventId}/reconciliation", fundingHandler.Reconcile)
		}
	}
	router.Handle("/", httptransport.NewHealthHandler(readiness))
	handler := middleware.Correlation(middleware.Contract(configuration.Environment, telemetry.HTTP(router)))
	server := &http.Server{
		Addr: configuration.HTTPAddress, Handler: handler,
		ReadHeaderTimeout: configuration.HTTPReadHeaderTimeout,
		ReadTimeout:       configuration.HTTPReadTimeout,
		WriteTimeout:      configuration.HTTPWriteTimeout,
		IdleTimeout:       configuration.HTTPIdleTimeout,
		MaxHeaderBytes:    configuration.HTTPMaxHeaderBytes,
	}
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

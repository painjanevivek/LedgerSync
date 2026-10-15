package integration_test

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/accounts"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/consistency"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/developerplatform"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/funding"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/reconciliation"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/transfers"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/cache"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/db"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/identity"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/transport/http/handlers"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/transport/http/middleware"
	"github.com/redis/go-redis/v9"
)

type exampleProviders []identity.DevelopmentProvider

func (providers exampleProviders) Authenticate(ctx context.Context, token string) (identity.Principal, error) {
	for _, provider := range providers {
		if principal, err := provider.Authenticate(ctx, token); err == nil {
			return principal, nil
		}
	}
	return identity.Principal{}, identity.ErrUnauthenticated
}

func exampleSecret(t *testing.T) string {
	t.Helper()
	value := make([]byte, 32)
	if _, err := rand.Read(value); err != nil {
		t.Fatal(err)
	}
	return hex.EncodeToString(value)
}

// This is an isolated synthetic identity issuer, not an OAuth provisioning API.
// Real handlers and RequestAuthenticator still enforce each HTTP principal.
func exampleAPIFixture(t *testing.T, admin *sql.DB) (*httptest.Server, string, map[string]map[string]string, *sql.DB) {
	t.Helper()
	ctx := context.Background()
	tenant := uuid.NewString()
	for _, query := range []string{
		`INSERT INTO tenants(id,external_reference) VALUES($1,'synthetic-independent-client')`,
		`INSERT INTO tenant_transfer_policies(tenant_id,currency,minimum_transfer_minor,maximum_transfer_minor,actor_rolling_24h_minor,source_account_rolling_24h_minor,tenant_rolling_24h_minor) VALUES($1,'INR',1,1000000000,5000000000,5000000000,10000000000)`,
		`INSERT INTO tenant_subject_roles(tenant_id,subject_id,role) VALUES($1,'sample-operator','operator'),($1,'sample-operator','finance'),($1,'sample-reviewer','finance'),($1,'sample-reader','operator')`,
		`INSERT INTO tenant_funding_policies(tenant_id,currency,mode,finance_activated,policy_version,per_command_minor,operator_rolling_24h_minor,tenant_rolling_24h_minor) VALUES($1,'INR','production_dual_control',true,1,1000000,1000000000,1000000000)`,
	} {
		if _, err := admin.ExecContext(ctx, query, tenant); err != nil {
			t.Fatal(err)
		}
	}
	api := provisionWorkloadSession(t, admin, testDatabaseURL(t), "ledgersync_api")
	api.db.SetMaxOpenConns(4)
	bindings := map[string]map[string]string{}
	providers := exampleProviders{}
	for _, alias := range []string{"operator", "reviewer", "reader"} {
		token := exampleSecret(t)
		bindings[alias] = map[string]string{"token": token}
		scopes := []string{"accounts:read"}
		switch alias {
		case "operator":
			scopes = []string{"accounts:read", "accounts:write", "funding:read", "funding:write", "funding:approve", "transfers:read", "transfers:write", "reconciliation:write", "webhooks:read", "webhooks:write"}
		case "reviewer":
			scopes = []string{"funding:read", "funding:write", "funding:approve"}
		}
		providers = append(providers, identity.DevelopmentProvider{SubjectID: "sample-" + alias, TenantID: tenant, Credential: token, Roles: []string{"tenant:operator", "tenant:finance"}, Scopes: scopes})
	}
	authenticator, err := identity.NewRequestAuthenticator(providers, exampleSecret(t))
	if err != nil {
		t.Fatal(err)
	}
	commandRepo, _ := db.NewAccountCommandRepository(api.db, time.Now)
	commandService, _ := accounts.NewCommandService(commandRepo, time.Now)
	accountHandler := handlers.NewAccountCommandHandler(commandService, providers).WithRequestAuthenticator(authenticator)
	fundingRepo, _ := db.NewFundingRepository(api.db, time.Now)
	fundingService, _ := funding.NewService(fundingRepo, funding.PolicyProductionDualControl, time.Now)
	fundingHandler := handlers.NewFundingHandler(fundingService, providers).WithRequestAuthenticator(authenticator)
	transferRepo, _ := db.NewTransferRepository(api.db, time.Now)
	transferRepo.WithPilotCurrency("INR")
	transferService, _ := transfers.NewService(transferRepo, time.Now)
	issuer, _ := consistency.NewIssuer(consistency.Key{ID: "example", Secret: []byte(exampleSecret(t))}, nil, nil, 10*time.Minute)
	fixed := time.Now()
	limiter, _ := db.NewRateLimitRepository(api.db, func() time.Time { return fixed })
	transferHandler := handlers.NewTransferHandler(transferService, providers, issuer).WithRequestAuthenticator(authenticator).WithRateLimiter(limiter, 8)
	client := redis.NewClient(&redis.Options{Addr: os.Getenv("LEDGERSYNC_TEST_REDIS_ADDR")})
	t.Cleanup(func() { _ = client.Close() })
	projection, _ := cache.NewBalanceCache(client, "example:", time.Minute)
	adapter, _ := cache.NewAccountAdapter(projection)
	balances, _ := db.NewBalanceRepository(api.db)
	reader, _ := accounts.NewReader(balances, adapter, issuer, accounts.ReaderConfig{})
	balanceHandler := handlers.NewBalanceHandler(reader, providers).WithRequestAuthenticator(authenticator)
	reconRepo, _ := db.NewReconciliationRepository(api.db)
	reconService, _ := reconciliation.NewCommandService(reconRepo, time.Now)
	reconHandler := handlers.NewReconciliationCommandHandler(reconService, providers).WithRequestAuthenticator(authenticator)
	webhookRepo, _ := db.NewDeveloperWebhookRepository(api.db, time.Now)
	webhookService, _ := developerplatform.NewWebhookService(webhookRepo, nil, "development", time.Now, nil)
	webhookHandler := handlers.NewDeveloperWebhookHandler(webhookService, providers).WithRequestAuthenticator(authenticator)
	router := http.NewServeMux()
	router.HandleFunc("POST /api/accounts", accountHandler.Create)
	router.HandleFunc("POST /api/funding-requests", fundingHandler.Request)
	router.HandleFunc("GET /api/funding-events/{fundingEventId}", fundingHandler.Get)
	router.HandleFunc("POST /api/funding-events/{fundingEventId}/approve", fundingHandler.Approve)
	router.HandleFunc("POST /api/funding-events/{fundingEventId}/post", fundingHandler.Post)
	router.HandleFunc("POST /api/transfers", transferHandler.ServeHTTP)
	router.Handle("GET /api/accounts/{accountID}/balance", balanceHandler)
	router.HandleFunc("POST /api/reconciliation/runs", reconHandler.Run)
	router.HandleFunc("POST /api/developer/webhooks", webhookHandler.Register)
	router.HandleFunc("GET /api/developer/webhooks/{webhookId}", webhookHandler.Get)
	server := httptest.NewServer(middleware.Correlation(middleware.Contract("development", router)))
	t.Cleanup(server.Close)
	return server, tenant, bindings, api.db
}

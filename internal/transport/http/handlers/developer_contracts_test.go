package handlers

import (
	"bytes"
	"context"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/operations"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	contractassets "github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/contracts"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/platform/identity"
)

type distributedReaderStub struct {
	tenant, actor string
	calls         int
}

func (s *distributedReaderStub) Read(_ context.Context, tenant, actor string) operations.DistributedEvidence {
	s.tenant, s.actor = tenant, actor
	s.calls++
	return operations.DistributedEvidence{}
}
func TestDistributedEvidenceRequiresScopeAndUsesVerifiedTenant(t *testing.T) {
	for _, scoped := range []bool{false, true} {
		scopes := []string{}
		if scoped {
			scopes = append(scopes, "developer:read")
		}
		reader := &distributedReaderStub{}
		handler := NewDeveloperContractHandler(identity.DevelopmentProvider{SubjectID: "reader", TenantID: "tenant-a", Scopes: scopes}).WithDistributedReader(reader)
		request := httptest.NewRequest(http.MethodGet, "/api/developer/distributed", nil)
		request.Header.Set("Authorization", "Bearer development-local-only")
		response := httptest.NewRecorder()
		handler.Distributed(response, request)
		if scoped {
			if response.Code != 200 || reader.actor != "reader" || reader.tenant != "tenant-a" || reader.calls != 1 {
				t.Fatal("authorized observation failed")
			}
		} else if response.Code != 403 || reader.calls != 0 {
			t.Fatal("unauthorized reader reached observations")
		}
	}
}

func TestDeveloperContractsReturnCanonicalEmbeddedBytesWithoutCredentials(t *testing.T) {
	handler := NewDeveloperContractHandler(identity.DevelopmentProvider{
		SubjectID: "developer", TenantID: "tenant-a", Scopes: []string{"developer:read"},
	})
	router := http.NewServeMux()
	router.HandleFunc("GET /api/developer/metadata", handler.Metadata)
	router.HandleFunc("GET /api/openapi.yaml", handler.OpenAPI)

	for _, testCase := range []struct {
		path, contentType, disposition string
		want                           string
	}{
		{path: "/api/developer/metadata", contentType: "application/json; charset=utf-8", want: contractassets.DeveloperExamplesV1()},
		{path: "/api/openapi.yaml", contentType: "application/yaml; charset=utf-8", disposition: `attachment; filename="ledgersync-openapi-` + contractassets.Version + `.yaml"`, want: contractassets.OpenAPIYAML()},
	} {
		request := httptest.NewRequest(http.MethodGet, testCase.path, nil)
		request.Header.Set("Authorization", "Bearer development-local-only")
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		if response.Code != http.StatusOK || response.Header().Get("Cache-Control") != "no-store" || response.Header().Get("X-Content-Type-Options") != "nosniff" {
			t.Fatalf("%s status=%d headers=%v", testCase.path, response.Code, response.Header())
		}
		if response.Header().Get("Content-Type") != testCase.contentType || response.Header().Get("Content-Disposition") != testCase.disposition {
			t.Fatalf("%s content headers=%v", testCase.path, response.Header())
		}
		if !bytes.Equal(response.Body.Bytes(), []byte(testCase.want)) {
			t.Fatalf("%s did not return canonical embedded bytes", testCase.path)
		}
		lower := strings.ToLower(response.Body.String())
		for _, forbidden := range []string{"authorization\"", "bearer development", "database_url", "session_secret", "cookie\""} {
			if strings.Contains(lower, forbidden) {
				t.Fatalf("%s exposed forbidden credential/infrastructure marker %q", testCase.path, forbidden)
			}
		}
	}
}

func TestDeveloperContractsRequireReadScopeAndGET(t *testing.T) {
	withoutScope := NewDeveloperContractHandler(identity.DevelopmentProvider{SubjectID: "reader", TenantID: "tenant-a"})
	request := httptest.NewRequest(http.MethodGet, "/api/developer/metadata", nil)
	request.Header.Set("Authorization", "Bearer development-local-only")
	response := httptest.NewRecorder()
	withoutScope.Metadata(response, request)
	if response.Code != http.StatusForbidden || response.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("missing scope status=%d headers=%v body=%s", response.Code, response.Header(), response.Body.String())
	}

	withScope := NewDeveloperContractHandler(identity.DevelopmentProvider{SubjectID: "reader", TenantID: "tenant-a", Scopes: []string{"developer:read"}})
	post := httptest.NewRequest(http.MethodPost, "/api/developer/metadata", nil)
	post.Header.Set("Authorization", "Bearer development-local-only")
	methodResponse := httptest.NewRecorder()
	withScope.Metadata(methodResponse, post)
	if methodResponse.Code != http.StatusMethodNotAllowed || methodResponse.Header().Get("Allow") != http.MethodGet {
		t.Fatalf("method status=%d headers=%v", methodResponse.Code, methodResponse.Header())
	}
}

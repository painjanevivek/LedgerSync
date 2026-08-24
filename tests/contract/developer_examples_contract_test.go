package contract_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	contractassets "github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/contracts"
)

type developerMetadata struct {
	SchemaVersion   string `json:"schema_version"`
	ContractVersion string `json:"contract_version"`
	BaseURL         string `json:"base_url"`
	OpenAPIPath     string `json:"openapi_download_path"`
	EndpointGroups  []struct {
		Operations []struct {
			OperationID string `json:"operation_id"`
			Method      string `json:"method"`
			Path        string `json:"path"`
			Scope       string `json:"scope"`
		} `json:"operations"`
	} `json:"endpoint_groups"`
	Examples []struct {
		ID            string         `json:"id"`
		OperationID   string         `json:"operation_id"`
		RequestSchema string         `json:"request_schema"`
		Method        string         `json:"method"`
		Path          string         `json:"path"`
		Headers       map[string]any `json:"headers"`
		Body          map[string]any `json:"body"`
		ResultFacts   map[string]any `json:"result_facts"`
	} `json:"examples"`
}

func TestDeveloperMetadataValidatesAgainstOpenAPIAndContainsNoSecrets(t *testing.T) {
	path := filepath.Join(repositoryRoot(t), "contracts", "developer-examples.v1.json")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(content) > 64*1024 {
		t.Fatalf("developer metadata is unbounded: %d bytes", len(content))
	}
	var raw map[string]any
	if err := json.Unmarshal(content, &raw); err != nil {
		t.Fatalf("developer metadata is not strict JSON: %v", err)
	}
	var metadata developerMetadata
	if err := json.Unmarshal(content, &metadata); err != nil {
		t.Fatal(err)
	}
	document := loadOpenAPIDocument(t)
	if err := validateOpenAPIValue(document, schemaAt(t, document, "DeveloperMetadata"), raw, "metadata"); err != nil {
		t.Fatal(err)
	}
	if metadata.SchemaVersion != "1" || metadata.ContractVersion != contractassets.Version || metadata.ContractVersion != stringAt(t, objectAt(t, document, "info"), "version") || metadata.BaseURL != "/api" || metadata.OpenAPIPath != "/api/openapi.yaml" {
		t.Fatalf("metadata identity drifted: %+v", metadata)
	}
	assertNoDeveloperSecrets(t, raw, "metadata")
	assertNoNumericJSONMoney(t, raw, "metadata")
}

func TestDeveloperEndpointCatalogueAndExamplesTrackOpenAPI(t *testing.T) {
	content, err := os.ReadFile(filepath.Join(repositoryRoot(t), "contracts", "developer-examples.v1.json"))
	if err != nil {
		t.Fatal(err)
	}
	var metadata developerMetadata
	if err := json.Unmarshal(content, &metadata); err != nil {
		t.Fatal(err)
	}
	document := loadOpenAPIDocument(t)
	contractOperations := map[string]bool{}
	operationByID := map[string]struct {
		method, path string
		operation    map[string]any
	}{}
	for path, rawPathItem := range objectAt(t, document, "paths") {
		for method, rawOperation := range asObject(t, rawPathItem) {
			if !openAPIMethods[method] {
				continue
			}
			operation := asObject(t, rawOperation)
			operationID := stringAt(t, operation, "operationId")
			key := strings.ToUpper(method) + " " + path
			contractOperations[key] = true
			operationByID[operationID] = struct {
				method, path string
				operation    map[string]any
			}{strings.ToUpper(method), path, operation}
		}
	}
	metadataOperations := map[string]bool{}
	for _, group := range metadata.EndpointGroups {
		for _, operation := range group.Operations {
			key := operation.Method + " " + operation.Path
			metadataOperations[key] = true
			contract, ok := operationByID[operation.OperationID]
			if !ok || contract.method != operation.Method || contract.path != operation.Path || contract.operation["x-required-scope"] != operation.Scope {
				t.Errorf("metadata operation %q points to %s, OpenAPI has %+v", operation.OperationID, key, contract)
			}
		}
	}
	assertStringSetsEqual(t, "developer metadata/OpenAPI operations", contractOperations, metadataOperations)

	for _, example := range metadata.Examples {
		contract, ok := operationByID[example.OperationID]
		if !ok || contract.method != example.Method || contract.path != example.Path {
			t.Fatalf("example %q operation drift: %+v", example.ID, contract)
		}
		requestBody := objectAt(t, contract.operation, "requestBody")
		mediaType := objectAt(t, objectAt(t, requestBody, "content"), "application/json")
		requestSchema := objectAt(t, mediaType, "schema")
		if requestSchema["$ref"] != example.RequestSchema {
			t.Fatalf("example %q schema=%v, OpenAPI=%v", example.ID, example.RequestSchema, requestSchema["$ref"])
		}
		resolved, resolveErr := resolveOpenAPIRef(document, example.RequestSchema)
		if resolveErr != nil {
			t.Fatal(resolveErr)
		}
		if err := validateOpenAPIValue(document, asObject(t, resolved), example.Body, "example."+example.ID+".body"); err != nil {
			t.Fatal(err)
		}
		if _, exists := example.Headers["Authorization"]; exists {
			t.Fatalf("example %q embeds an Authorization header", example.ID)
		}
	}
}

func TestDeveloperZeroAccountFactsMatchCanonicalResponseExample(t *testing.T) {
	content, err := os.ReadFile(filepath.Join(repositoryRoot(t), "contracts", "developer-examples.v1.json"))
	if err != nil {
		t.Fatal(err)
	}
	var metadata developerMetadata
	if err := json.Unmarshal(content, &metadata); err != nil {
		t.Fatal(err)
	}
	document := loadOpenAPIDocument(t)
	accountCreate := objectAt(t, objectAt(t, objectAt(t, document, "components"), "examples"), "AccountCreateOriginal")
	canonical := objectAt(t, accountCreate, "value")
	for _, example := range metadata.Examples {
		if example.ID != "create_account" {
			continue
		}
		for _, field := range []string{"currency", "status", "available_minor", "ledger_minor"} {
			if example.ResultFacts[field] != canonical[field] {
				t.Fatalf("create_account result_facts.%s=%v, canonical=%v", field, example.ResultFacts[field], canonical[field])
			}
		}
		if example.ResultFacts["available_minor"] != "0" || example.ResultFacts["ledger_minor"] != "0" {
			t.Fatalf("new account exact-zero facts drifted: %#v", example.ResultFacts)
		}
		return
	}
	t.Fatal("create_account example is missing")
}

func TestNamedOpenAPIExamplesValidateAgainstTheirSchemas(t *testing.T) {
	document := loadOpenAPIDocument(t)
	examples := objectAt(t, objectAt(t, document, "components"), "examples")
	for name, schemaName := range map[string]string{
		"AccountCreateOriginal":         "AccountCommandResult",
		"AccountCreateReplay":           "AccountCommandResult",
		"InvalidAccountTransition":      "Error",
		"AccountVersionConflict":        "Error",
		"ExternalReferenceConflict":     "Error",
		"AccountNotZero":                "Error",
		"ReadOnlyDenial":                "Error",
		"AccountResponseUnknown":        "Error",
		"ReconciliationResponseUnknown": "Error",
	} {
		example := objectAt(t, examples, name)
		if err := validateOpenAPIValue(document, schemaAt(t, document, schemaName), example["value"], "components.examples."+name); err != nil {
			t.Error(err)
		}
	}
	conflictResponse := objectAt(t, objectAt(t, document, "components"), "responses")["ReconciliationConflict"]
	conflictSchema := objectAt(t, objectAt(t, objectAt(t, asObject(t, conflictResponse), "content"), "application/json"), "schema")
	if err := validateOpenAPIValue(document, conflictSchema, objectAt(t, examples, "ReconciliationAlreadyRunning")["value"], "components.examples.ReconciliationAlreadyRunning"); err != nil {
		t.Error(err)
	}
}

func TestDeveloperContractSurfaceHasNoRunnerOrCredentialEndpoint(t *testing.T) {
	document := loadOpenAPIDocument(t)
	paths := objectAt(t, document, "paths")
	for path, rawPathItem := range paths {
		lower := strings.ToLower(path)
		for _, forbidden := range []string{"credential", "token", "secret", "execute", "request-runner", "proxy"} {
			if strings.Contains(lower, forbidden) {
				t.Errorf("OpenAPI exposes forbidden developer surface %q", path)
			}
		}
		if path == "/developer/metadata" || path == "/openapi.yaml" {
			for method := range asObject(t, rawPathItem) {
				if openAPIMethods[method] && method != "get" {
					t.Errorf("developer contract route %s exposes mutation %s", path, method)
				}
			}
		}
	}
	handler, err := os.ReadFile(filepath.Join(repositoryRoot(t), "internal", "transport", "http", "handlers", "developer_contracts.go"))
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"http.Client", ".Do(", "request.URL.Query", "request.FormValue", "Authorization\",", "Credential"} {
		if strings.Contains(string(handler), forbidden) {
			t.Errorf("developer contract handler contains request-runner/credential primitive %q", forbidden)
		}
	}
}

func validateOpenAPIValue(document, schema map[string]any, value any, path string) error {
	if ref, ok := schema["$ref"].(string); ok {
		resolved, err := resolveOpenAPIRef(document, ref)
		if err != nil {
			return err
		}
		return validateOpenAPIValue(document, resolved.(map[string]any), value, path)
	}
	if allOf, ok := schema["allOf"].([]any); ok {
		for _, item := range allOf {
			if err := validateOpenAPIValue(document, item.(map[string]any), value, path); err != nil {
				return err
			}
		}
	}
	typeName, _ := schema["type"].(string)
	switch typeName {
	case "object":
		object, ok := value.(map[string]any)
		if !ok {
			return fmt.Errorf("%s is %T, want object", path, value)
		}
		properties, _ := schema["properties"].(map[string]any)
		if required, ok := schema["required"].([]any); ok {
			for _, rawName := range required {
				name := rawName.(string)
				if _, exists := object[name]; !exists {
					return fmt.Errorf("%s missing required property %s", path, name)
				}
			}
		}
		if schema["additionalProperties"] == false {
			for name := range object {
				if _, exists := properties[name]; !exists {
					return fmt.Errorf("%s has unknown property %s", path, name)
				}
			}
		}
		if minimum, ok := numericSchemaValue(schema["minProperties"]); ok && len(object) < minimum {
			return fmt.Errorf("%s has fewer than %d properties", path, minimum)
		}
		if maximum, ok := numericSchemaValue(schema["maxProperties"]); ok && len(object) > maximum {
			return fmt.Errorf("%s has more than %d properties", path, maximum)
		}
		for name, child := range object {
			if childSchema, exists := properties[name].(map[string]any); exists {
				if err := validateOpenAPIValue(document, childSchema, child, path+"."+name); err != nil {
					return err
				}
			}
		}
	case "array":
		array, ok := value.([]any)
		if !ok {
			return fmt.Errorf("%s is %T, want array", path, value)
		}
		if minimum, ok := numericSchemaValue(schema["minItems"]); ok && len(array) < minimum {
			return fmt.Errorf("%s has fewer than %d items", path, minimum)
		}
		if maximum, ok := numericSchemaValue(schema["maxItems"]); ok && len(array) > maximum {
			return fmt.Errorf("%s has more than %d items", path, maximum)
		}
		if itemSchema, ok := schema["items"].(map[string]any); ok {
			for index, item := range array {
				if err := validateOpenAPIValue(document, itemSchema, item, fmt.Sprintf("%s[%d]", path, index)); err != nil {
					return err
				}
			}
		}
	case "string":
		text, ok := value.(string)
		if !ok {
			return fmt.Errorf("%s is %T, want string", path, value)
		}
		if minimum, ok := numericSchemaValue(schema["minLength"]); ok && utf8.RuneCountInString(text) < minimum {
			return fmt.Errorf("%s is shorter than %d", path, minimum)
		}
		if maximum, ok := numericSchemaValue(schema["maxLength"]); ok && utf8.RuneCountInString(text) > maximum {
			return fmt.Errorf("%s is longer than %d", path, maximum)
		}
		if pattern, ok := schema["pattern"].(string); ok {
			compiled, err := regexp.Compile(pattern)
			if err != nil || !compiled.MatchString(text) {
				return fmt.Errorf("%s does not match %q", path, pattern)
			}
		}
		if enum, ok := schema["enum"].([]any); ok {
			matched := false
			for _, allowed := range enum {
				matched = matched || allowed == text
			}
			if !matched {
				return fmt.Errorf("%s=%q is outside enum", path, text)
			}
		}
		switch schema["format"] {
		case "uuid":
			if !regexp.MustCompile(`^[0-9A-Fa-f]{8}-[0-9A-Fa-f]{4}-[0-9A-Fa-f]{4}-[0-9A-Fa-f]{4}-[0-9A-Fa-f]{12}$`).MatchString(text) {
				return fmt.Errorf("%s=%q is not a UUID", path, text)
			}
		case "date-time":
			if _, err := time.Parse(time.RFC3339Nano, text); err != nil {
				return fmt.Errorf("%s=%q is not RFC3339: %w", path, text, err)
			}
		}
	case "boolean":
		if _, ok := value.(bool); !ok {
			return fmt.Errorf("%s is %T, want boolean", path, value)
		}
	}
	return nil
}

func numericSchemaValue(value any) (int, bool) {
	switch number := value.(type) {
	case int:
		return number, true
	case float64:
		return int(number), true
	default:
		return 0, false
	}
}

func assertNoDeveloperSecrets(t *testing.T, value any, path string) {
	t.Helper()
	switch typed := value.(type) {
	case map[string]any:
		for key, child := range typed {
			lowerKey := strings.ToLower(key)
			for _, forbidden := range []string{"authorization", "cookie", "csrf_token", "access_token", "credential_value", "secret", "password", "database_url", "redis_url"} {
				if lowerKey == forbidden {
					t.Errorf("%s contains forbidden secret-bearing field %q", path, key)
				}
			}
			assertNoDeveloperSecrets(t, child, path+"."+key)
		}
	case []any:
		for index, child := range typed {
			assertNoDeveloperSecrets(t, child, fmt.Sprintf("%s[%d]", path, index))
		}
	case string:
		lower := strings.ToLower(typed)
		for _, forbidden := range []string{"bearer ", "postgres://", "redis://", "password=", "session="} {
			if strings.Contains(lower, forbidden) {
				t.Errorf("%s contains forbidden secret value pattern %q", path, forbidden)
			}
		}
	}
}

func assertNoNumericJSONMoney(t *testing.T, value any, path string) {
	t.Helper()
	switch typed := value.(type) {
	case map[string]any:
		for key, child := range typed {
			lower := strings.ToLower(key)
			if strings.Contains(lower, "amount") || strings.Contains(lower, "minor") || strings.Contains(lower, "balance") {
				if _, numeric := child.(float64); numeric {
					t.Errorf("%s.%s uses numeric JSON money", path, key)
				}
			}
			assertNoNumericJSONMoney(t, child, path+"."+key)
		}
	case []any:
		for index, child := range typed {
			assertNoNumericJSONMoney(t, child, fmt.Sprintf("%s[%d]", path, index))
		}
	}
}

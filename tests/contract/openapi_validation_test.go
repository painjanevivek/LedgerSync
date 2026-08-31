package contract_test

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

var openAPIMethods = map[string]bool{"get": true, "post": true, "patch": true, "put": true, "delete": true}

func loadOpenAPIDocument(t *testing.T) map[string]any {
	t.Helper()
	content, err := os.ReadFile(filepath.Join(repositoryRoot(t), "contracts", "openapi.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	var document map[string]any
	if err := yaml.Unmarshal(content, &document); err != nil {
		t.Fatalf("OpenAPI is not valid YAML: %v", err)
	}
	return document
}

func TestOpenAPIIsStructurallyValidAndEveryReferenceResolves(t *testing.T) {
	root := repositoryRoot(t)
	content, err := os.ReadFile(filepath.Join(root, "contracts", "openapi.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if len(content) > 160*1024 {
		t.Fatalf("OpenAPI download is unbounded: %d bytes", len(content))
	}
	document := loadOpenAPIDocument(t)
	if document["openapi"] != "3.1.0" {
		t.Fatalf("openapi=%v, want 3.1.0", document["openapi"])
	}
	info := objectAt(t, document, "info")
	if strings.TrimSpace(stringAt(t, info, "title")) == "" || strings.TrimSpace(stringAt(t, info, "version")) == "" {
		t.Fatal("OpenAPI info title/version are required")
	}
	servers := arrayAt(t, document, "servers")
	if len(servers) != 1 || stringAt(t, asObject(t, servers[0]), "url") != "/api" {
		t.Fatalf("OpenAPI must expose exactly the private /api server, got %#v", servers)
	}

	walkOpenAPI(t, document, func(value map[string]any) {
		if ref, ok := value["$ref"].(string); ok {
			if _, err := resolveOpenAPIRef(document, ref); err != nil {
				t.Errorf("unresolved OpenAPI reference %q: %v", ref, err)
			}
		}
	})

	paths := objectAt(t, document, "paths")
	operationIDs := map[string]string{}
	pathVariable := regexp.MustCompile(`\{([^}]+)\}`)
	for path, rawPathItem := range paths {
		pathItem := asObject(t, rawPathItem)
		for method, rawOperation := range pathItem {
			if !openAPIMethods[method] {
				continue
			}
			operation := asObject(t, rawOperation)
			operationID := stringAt(t, operation, "operationId")
			requiredScope := stringAt(t, operation, "x-required-scope")
			if !regexp.MustCompile(`^[a-z]+:[a-z]+$`).MatchString(requiredScope) {
				t.Errorf("%s %s has invalid required scope %q", strings.ToUpper(method), path, requiredScope)
			}
			if previous, exists := operationIDs[operationID]; operationID == "" || exists {
				t.Errorf("operationId %q at %s %s is empty or duplicates %s", operationID, strings.ToUpper(method), path, previous)
			} else {
				operationIDs[operationID] = strings.ToUpper(method) + " " + path
			}
			responses := objectAt(t, operation, "responses")
			hasSuccess := false
			for status := range responses {
				hasSuccess = hasSuccess || strings.HasPrefix(status, "2")
			}
			if !hasSuccess {
				t.Errorf("%s %s has no 2xx response", strings.ToUpper(method), path)
			}
			declared := declaredPathParameters(t, document, pathItem, operation)
			for _, match := range pathVariable.FindAllStringSubmatch(path, -1) {
				if !declared[match[1]] {
					t.Errorf("%s %s does not declare required path parameter %q", strings.ToUpper(method), path, match[1])
				}
			}
		}
	}
}

func TestOpenAPIRoutesExactlyMatchRegisteredPrivateAPI(t *testing.T) {
	document := loadOpenAPIDocument(t)
	contractRoutes := map[string]bool{}
	for path, rawPathItem := range objectAt(t, document, "paths") {
		for method := range asObject(t, rawPathItem) {
			if openAPIMethods[method] {
				contractRoutes[strings.ToUpper(method)+" "+path] = true
			}
		}
	}

	runtimeRoutes := map[string]bool{}
	routePattern := regexp.MustCompile(`(?:Handle|HandleFunc)\("((?:GET|POST|PATCH|PUT|DELETE) /api/[^" ]+)"`)
	apiDirectory := filepath.Join(repositoryRoot(t), "cmd", "api")
	entries, err := os.ReadDir(apiDirectory)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		content, readErr := os.ReadFile(filepath.Join(apiDirectory, entry.Name()))
		if readErr != nil {
			t.Fatal(readErr)
		}
		for _, match := range routePattern.FindAllStringSubmatch(string(content), -1) {
			route := strings.TrimPrefix(match[1], strings.Fields(match[1])[0]+" /api")
			method := strings.Fields(match[1])[0]
			route = strings.NewReplacer("{accountID}", "{accountId}", "{transferID}", "{transferId}", "{runID}", "{runId}", "{eventID}", "{eventId}").Replace(route)
			runtimeRoutes[method+" "+route] = true
		}
	}
	assertStringSetsEqual(t, "OpenAPI/runtime routes", contractRoutes, runtimeRoutes)
}

func TestTransferHistoryFiltersAreServerSideBoundedAndCursorBound(t *testing.T) {
	document := loadOpenAPIDocument(t)
	operation := objectAt(t, asObject(t, objectAt(t, document, "paths")["/transfers"]), "get")
	parameters := map[string]map[string]any{}
	for _, raw := range arrayAt(t, operation, "parameters") {
		parameter := asObject(t, raw)
		if name, ok := parameter["name"].(string); ok {
			parameters[name] = parameter
		}
	}
	query := parameters["q"]
	querySchema := objectAt(t, query, "schema")
	if querySchema["maxLength"] != 128 || querySchema["pattern"] != "^[0-9A-Fa-f-]+$" || !strings.Contains(stringAt(t, query, "description"), "cursor is bound") {
		t.Fatalf("transfer q contract is not bounded/cursor-bound: %#v", query)
	}
	status := parameters["status"]
	statusSchema := objectAt(t, status, "schema")
	gotStatuses := map[string]bool{}
	for _, raw := range arrayAt(t, statusSchema, "enum") {
		gotStatuses[fmt.Sprint(raw)] = true
	}
	if !reflect.DeepEqual(gotStatuses, map[string]bool{"pending": true, "posted": true, "rejected": true}) || !strings.Contains(stringAt(t, status, "description"), "before pagination") {
		t.Fatalf("transfer status contract is not server-side and exact: %#v", status)
	}
}

func TestOpenAPIResponseSchemasMatchRuntimeJSONDTOs(t *testing.T) {
	document := loadOpenAPIDocument(t)
	tests := []struct{ file, structName, schema string }{
		{"internal/transport/http/handlers/accounts.go", "accountResponse", "Account"},
		{"internal/transport/http/handlers/account_commands.go", "accountCommandResponse", "AccountCommandResult"},
		{"internal/transport/http/handlers/balances.go", "balanceResponse", "Balance"},
		{"internal/transport/http/handlers/reconciliation_commands.go", "reconciliationCommandResponse", "ReconciliationRun"},
		{"internal/application/transfers/service.go", "resultJSON", "TransferResult"},
		{"internal/application/transfers/service.go", "balanceJSON", "TransferBalance"},
		{"internal/application/transactions/history.go", "Entry", "TransactionEntry"},
		{"internal/application/investigation/models.go", "TransferSummary", "TransferSummary"},
		{"internal/application/investigation/models.go", "Posting", "Posting"},
		{"internal/application/investigation/models.go", "ReconciliationMismatch", "ReconciliationMismatch"},
		{"internal/application/investigation/models.go", "ReconciliationRun", "ReconciliationRun"},
		{"internal/application/investigation/models.go", "SearchResult", "SearchResult"},
		{"internal/application/investigation/models.go", "SearchPage", "InvestigationSearchPage"},
		{"internal/application/investigation/models.go", "Relationship", "Relationship"},
		{"internal/application/investigation/models.go", "RelationshipPage", "RelationshipPage"},
		{"internal/application/investigation/saved_views.go", "SavedView", "SavedInvestigationView"},
		{"internal/application/investigation/saved_views.go", "SavedViewPage", "SavedInvestigationViewPage"},
		{"internal/application/operations/diagnostics.go", "DiagnosticSnapshot", "DiagnosticSnapshot"},
		{"internal/application/operations/events.go", "EventEvidence", "EventEvidence"},
		{"internal/application/operations/events.go", "DeliveryEvidence", "DeliveryEvidence"},
		{"internal/application/operations/events.go", "EventTimelineItem", "EventTimelineItem"},
		{"internal/application/recovery/manifests.go", "ManifestSnapshot", "RecoveryEvidenceIndex"},
		{"internal/application/recovery/manifests.go", "BackupManifestEvidence", "RecoveryBackupEvidence"},
		{"internal/application/recovery/manifests.go", "RestoreManifestEvidence", "RecoveryRestoreEvidence"},
		{"internal/application/recovery/manifests.go", "ManifestRetention", "RecoveryRetentionEvidence"},
		{"internal/application/guidance/service.go", "OrientationSummary", "OrientationSummary"},
		{"internal/application/guidance/service.go", "OrientationStep", "OrientationStep"},
		{"internal/application/guidance/service.go", "ExplainabilityTimeline", "ExplainabilityTimeline"},
		{"internal/application/guidance/service.go", "TimelineStage", "ExplainabilityStage"},
		{"internal/application/guidance/service.go", "EvidenceItem", "ExplainabilityEvidence"},
	}
	for _, testCase := range tests {
		t.Run(testCase.schema, func(t *testing.T) {
			jsonFields := goStructJSONFields(t, filepath.Join(repositoryRoot(t), filepath.FromSlash(testCase.file)), testCase.structName)
			schema := schemaAt(t, document, testCase.schema)
			schemaFields := flattenedSchemaProperties(t, document, schema)
			requiredFields := flattenedSchemaRequired(t, document, schema)
			for field := range requiredFields {
				if !jsonFields[field] {
					t.Errorf("%s/%s runtime DTO is missing required schema field %q", testCase.structName, testCase.schema, field)
				}
			}
			for field := range jsonFields {
				if !schemaFields[field] {
					t.Errorf("%s/%s runtime DTO exposes undocumented field %q", testCase.structName, testCase.schema, field)
				}
			}
		})
	}
}

func declaredPathParameters(t *testing.T, document, pathItem, operation map[string]any) map[string]bool {
	t.Helper()
	result := map[string]bool{}
	for _, source := range []map[string]any{pathItem, operation} {
		raw, ok := source["parameters"]
		if !ok {
			continue
		}
		parameters, ok := raw.([]any)
		if !ok {
			t.Fatalf("parameters is %T", raw)
		}
		for _, item := range parameters {
			parameter := asObject(t, item)
			if ref, ok := parameter["$ref"].(string); ok {
				resolved, err := resolveOpenAPIRef(document, ref)
				if err != nil {
					t.Fatal(err)
				}
				parameter = asObject(t, resolved)
			}
			if parameter["in"] == "path" && parameter["required"] == true {
				result[stringAt(t, parameter, "name")] = true
			}
		}
	}
	return result
}

func walkOpenAPI(t *testing.T, value any, visit func(map[string]any)) {
	t.Helper()
	switch typed := value.(type) {
	case map[string]any:
		visit(typed)
		for _, child := range typed {
			walkOpenAPI(t, child, visit)
		}
	case []any:
		for _, child := range typed {
			walkOpenAPI(t, child, visit)
		}
	}
}

func resolveOpenAPIRef(document map[string]any, ref string) (any, error) {
	if !strings.HasPrefix(ref, "#/") {
		return nil, fmt.Errorf("only local references are allowed")
	}
	var current any = document
	for _, part := range strings.Split(strings.TrimPrefix(ref, "#/"), "/") {
		object, ok := current.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("%q is not an object", part)
		}
		current, ok = object[strings.ReplaceAll(strings.ReplaceAll(part, "~1", "/"), "~0", "~")]
		if !ok {
			return nil, fmt.Errorf("missing %q", part)
		}
	}
	return current, nil
}

func schemaAt(t *testing.T, document map[string]any, name string) map[string]any {
	t.Helper()
	return asObject(t, objectAt(t, objectAt(t, document, "components"), "schemas")[name])
}

func flattenedSchemaProperties(t *testing.T, document, schema map[string]any) map[string]bool {
	t.Helper()
	result := map[string]bool{}
	if ref, ok := schema["$ref"].(string); ok {
		resolved, err := resolveOpenAPIRef(document, ref)
		if err != nil {
			t.Fatal(err)
		}
		return flattenedSchemaProperties(t, document, asObject(t, resolved))
	}
	if properties, ok := schema["properties"].(map[string]any); ok {
		for name := range properties {
			result[name] = true
		}
	}
	if allOf, ok := schema["allOf"].([]any); ok {
		for _, item := range allOf {
			for name := range flattenedSchemaProperties(t, document, asObject(t, item)) {
				result[name] = true
			}
		}
	}
	return result
}

func flattenedSchemaRequired(t *testing.T, document, schema map[string]any) map[string]bool {
	t.Helper()
	result := map[string]bool{}
	if ref, ok := schema["$ref"].(string); ok {
		resolved, err := resolveOpenAPIRef(document, ref)
		if err != nil {
			t.Fatal(err)
		}
		return flattenedSchemaRequired(t, document, asObject(t, resolved))
	}
	if required, ok := schema["required"].([]any); ok {
		for _, item := range required {
			result[item.(string)] = true
		}
	}
	if allOf, ok := schema["allOf"].([]any); ok {
		for _, item := range allOf {
			for name := range flattenedSchemaRequired(t, document, asObject(t, item)) {
				result[name] = true
			}
		}
	}
	return result
}

func goStructJSONFields(t *testing.T, path, structName string) map[string]bool {
	t.Helper()
	parsed, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	result := map[string]bool{}
	found := false
	ast.Inspect(parsed, func(node ast.Node) bool {
		typeSpec, ok := node.(*ast.TypeSpec)
		if !ok || typeSpec.Name.Name != structName {
			return true
		}
		structure, ok := typeSpec.Type.(*ast.StructType)
		if !ok {
			t.Fatalf("%s is not a struct", structName)
		}
		found = true
		for _, field := range structure.Fields.List {
			if field.Tag == nil {
				continue
			}
			tag, unquoteErr := strconvUnquote(field.Tag.Value)
			if unquoteErr != nil {
				t.Fatal(unquoteErr)
			}
			name := strings.Split(reflect.StructTag(tag).Get("json"), ",")[0]
			if name != "" && name != "-" {
				result[name] = true
			}
		}
		return false
	})
	if !found {
		t.Fatalf("struct %s not found in %s", structName, path)
	}
	return result
}

func strconvUnquote(value string) (string, error) {
	if len(value) < 2 || value[0] != '`' || value[len(value)-1] != '`' {
		return "", fmt.Errorf("unexpected Go struct tag %q", value)
	}
	return value[1 : len(value)-1], nil
}

func assertStringSetsEqual(t *testing.T, label string, left, right map[string]bool) {
	t.Helper()
	missing, extra := []string{}, []string{}
	for item := range left {
		if !right[item] {
			missing = append(missing, item)
		}
	}
	for item := range right {
		if !left[item] {
			extra = append(extra, item)
		}
	}
	sort.Strings(missing)
	sort.Strings(extra)
	if len(missing) > 0 || len(extra) > 0 {
		t.Fatalf("%s drift: left-only=%v right-only=%v", label, missing, extra)
	}
}

func objectAt(t *testing.T, object map[string]any, key string) map[string]any {
	t.Helper()
	value, ok := object[key]
	if !ok {
		t.Fatalf("missing object %q", key)
	}
	return asObject(t, value)
}

func asObject(t *testing.T, value any) map[string]any {
	t.Helper()
	object, ok := value.(map[string]any)
	if !ok {
		t.Fatalf("expected object, got %T", value)
	}
	return object
}

func arrayAt(t *testing.T, object map[string]any, key string) []any {
	t.Helper()
	value, ok := object[key].([]any)
	if !ok {
		t.Fatalf("%q is %T, want array", key, object[key])
	}
	return value
}

func stringAt(t *testing.T, object map[string]any, key string) string {
	t.Helper()
	value, ok := object[key].(string)
	if !ok {
		t.Fatalf("%q is %T, want string", key, object[key])
	}
	return value
}

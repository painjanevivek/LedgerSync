package investigation

import (
	"reflect"
	"testing"
)

func TestSavedViewDefinitionsAreVersionedAllowlistedAndCanonical(t *testing.T) {
	filters, target, err := NormalizeSavedViewDefinition("transfers", 1, map[string]string{
		"status":    "posted",
		"accountId": "AAAAAAAA-AAAA-4AAA-8AAA-AAAAAAAAAAAA",
		"from":      "2026-08-01T00:00:00+00:00",
	})
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{"status": "posted", "accountId": "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa", "from": "2026-08-01T00:00:00Z"}
	if !reflect.DeepEqual(filters, want) || target != "/transfers?accountId=aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa&from=2026-08-01T00%3A00%3A00Z&status=posted" {
		t.Fatalf("filters=%#v target=%q", filters, target)
	}
	filters, _, err = NormalizeSavedViewDefinition("transfers", 1, map[string]string{"to": "2026-08-01T23:59:59.999Z"})
	if err != nil || filters["to"] != "2026-08-01T23:59:59.999Z" {
		t.Fatalf("fractional UTC boundary was not preserved: filters=%#v err=%v", filters, err)
	}

	invalid := []struct {
		domain  string
		version int
		filters map[string]string
	}{
		{"accounts", 2, map[string]string{"status": "active"}},
		{"accounts", 1, map[string]string{}},
		{"accounts", 1, map[string]string{"q": "possibly-secret-free-text"}},
		{"transfers", 1, map[string]string{"cursor": "snapshot-token"}},
		{"transfers", 1, map[string]string{"from": "2026-08-02T00:00:00Z", "to": "2026-08-01T00:00:00Z"}},
		{"approvals", 1, map[string]string{"domain": "funding", "status": "correction:requested"}},
		{"events", 1, map[string]string{"eventType": "event name with spaces"}},
		{"unknown", 1, map[string]string{"status": "active"}},
	}
	for _, testCase := range invalid {
		if _, _, err := NormalizeSavedViewDefinition(testCase.domain, testCase.version, testCase.filters); err == nil {
			t.Fatalf("accepted domain=%q version=%d filters=%#v", testCase.domain, testCase.version, testCase.filters)
		}
	}
}

func TestSavedViewNamesAndAccessAreBounded(t *testing.T) {
	if name, err := NormalizeSavedViewName("Dead delivery events"); err != nil || name != "Dead delivery events" {
		t.Fatalf("name=%q err=%v", name, err)
	}
	for _, value := range []string{"", " leading", "trailing ", "line\nbreak"} {
		if _, err := NormalizeSavedViewName(value); err == nil {
			t.Fatalf("accepted name %q", value)
		}
	}
	access := SavedViewAccess{FundingApprovals: true}
	if !SavedViewDefinitionAllowed("approvals", map[string]string{"domain": "funding"}, access) || SavedViewDefinitionAllowed("approvals", map[string]string{"domain": "correction"}, access) || SavedViewDefinitionAllowed("funding", map[string]string{"status": "approved"}, access) {
		t.Fatal("saved-view domain access did not preserve the exact approval boundary")
	}
}

package observability

import "testing"

func TestPerformanceRouteOperationIsBoundedAndNeverUsesRawObjectPaths(t *testing.T) {
	tests := map[string]string{
		"POST /api/transfers":                   "transfer_command",
		"GET /api/accounts/{accountID}/balance": "balance_read",
		"GET /api/local/diagnostics":            "diagnostics_read",
		"GET /api/events":                       "events_list",
		"GET /api/events/{eventID}":             "events_detail",
		"POST /api/accounts":                    "account_create",
		"PATCH /api/accounts/{accountID}":       "account_patch",
	}
	for pattern, want := range tests {
		if got := performanceRouteOperation(pattern); got != want {
			t.Errorf("operation(%q)=%q, want %q", pattern, got, want)
		}
	}
	for _, unapproved := range []string{"", "GET /api/accounts/secret-account/balance", "GET /api/events/secret-event", "GET /api/events/{eventID}/payload"} {
		if got := performanceRouteOperation(unapproved); got != "" {
			t.Errorf("unapproved pattern %q produced metric label %q", unapproved, got)
		}
	}
}

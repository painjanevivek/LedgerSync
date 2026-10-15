package db

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

type observationTransport func(*http.Request) (*http.Response, error)

func (f observationTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}
func TestDistributedEvidenceDoesNotInferMissingReplicaCount(t *testing.T) {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	for _, test := range []struct{ name, body, wantState, wantCount string }{
		{"valid", `{"captured_at":"` + now + `","ready_api_count":2}`, "available", "2"},
		{"missing count", `{"captured_at":"` + now + `"}`, "unavailable", ""},
		{"oversized", `{"captured_at":"` + now + `","ready_api_count":2,"padding":"` + strings.Repeat("a", 4096) + `"}`, "unavailable", ""},
		{"stale", `{"captured_at":"2020-01-01T00:00:00Z","ready_api_count":2}`, "unavailable", ""},
		{"trailing oversized body", `{"captured_at":"` + now + `","ready_api_count":2}` + strings.Repeat(" ", 4096), "unavailable", ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			repository := NewDistributedEvidenceRepository(nil, true)
			repository.client = &http.Client{Transport: observationTransport(func(*http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(test.body))}, nil
			})}
			result := repository.Read(context.Background(), "tenant-a", "actor")
			if result.APIReplicas.State != test.wantState || result.APIReplicas.Count != test.wantCount {
				t.Fatalf("replica observation %+v", result.APIReplicas)
			}
			if result.WorkerPublications.State != "unavailable" || result.WorkerPublications.Count != "" || result.Scheduler.State != "unavailable" {
				t.Fatal("missing database was reported as observed")
			}
		})
	}
}

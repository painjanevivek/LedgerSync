package operations

import (
	"context"
	"errors"
	"testing"
	"time"
)

type webhookEndpointRepositoryStub struct {
	filter WebhookEndpointFilter
}

func (r *webhookEndpointRepositoryStub) ListWebhookEndpoints(_ context.Context, _, _ string, filter WebhookEndpointFilter) (WebhookEndpointPage, error) {
	r.filter = filter
	return WebhookEndpointPage{Items: []WebhookEndpointEvidence{{EndpointID: "70000000-0000-4000-8000-000000000001", Label: " Accounting ", EndpointURL: "https://partner.example.test/private/hook?token=hidden", Status: "active", SubscribedEvents: []string{"transfer.posted"}, RecentDeliveryState: "delivered", RecentAttemptCount: "2", RecentDeadCount: "0", UpdatedAt: time.Date(2026, 8, 31, 1, 0, 0, 0, time.FixedZone("test", 3600))}}}, nil
}
func (r *webhookEndpointRepositoryStub) GetWebhookEndpoint(context.Context, string, string, string) (WebhookEndpointDetail, error) {
	return WebhookEndpointDetail{WebhookEndpointEvidence: WebhookEndpointEvidence{EndpointID: "70000000-0000-4000-8000-000000000001", Label: "bad\x00label", EndpointURL: "https://user:secret@partner.example.test/hooks", Status: "active", SubscribedEvents: []string{"transfer.posted"}, RecentDeliveryState: "none", RecentAttemptCount: "0", RecentDeadCount: "0", UpdatedAt: time.Now()}}, nil
}

func TestWebhookEndpointServiceReturnsOnlySafeEndpointEvidence(t *testing.T) {
	repository := &webhookEndpointRepositoryStub{}
	service, err := NewWebhookEndpointService(repository)
	if err != nil {
		t.Fatal(err)
	}
	page, err := service.List(context.Background(), "tenant", "actor", WebhookEndpointFilter{Status: "ACTIVE", EventType: "transfer.posted", Limit: 25})
	if err != nil || len(page.Items) != 1 {
		t.Fatalf("page=%+v error=%v", page, err)
	}
	item := page.Items[0]
	if repository.filter.Status != "active" || item.Label != "Accounting" || item.Origin != "https://partner.example.test" || item.EndpointURL != "" || !item.UpdatedAt.Equal(time.Date(2026, 8, 31, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("unsafe or unnormalized endpoint=%+v filter=%+v", item, repository.filter)
	}
	detail, err := service.Get(context.Background(), "tenant", "actor", "70000000-0000-4000-8000-000000000001")
	if err != nil || detail.Label != "Webhook endpoint" || detail.Origin != "unavailable" || detail.EndpointURL != "" {
		t.Fatalf("unsafe detail=%+v error=%v", detail, err)
	}
}

func TestWebhookEndpointServiceRejectsUnboundedFilters(t *testing.T) {
	service, _ := NewWebhookEndpointService(&webhookEndpointRepositoryStub{})
	for name, filter := range map[string]WebhookEndpointFilter{
		"status": {Status: "deleted", Limit: 25},
		"event":  {EventType: "transfer posted", Limit: 25},
		"limit":  {Limit: 101},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := service.List(context.Background(), "tenant", "actor", filter); !errors.Is(err, ErrInvalidWebhookEvidence) {
				t.Fatalf("error=%v", err)
			}
		})
	}
}

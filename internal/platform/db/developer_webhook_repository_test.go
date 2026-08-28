package db

import "testing"

type webhookScanFunc func(...any) error

func (fn webhookScanFunc) Scan(destination ...any) error { return fn(destination...) }

func TestScanWebhookDecodesDatabaseArrayProjection(t *testing.T) {
	item, err := scanWebhook(webhookScanFunc(func(destination ...any) error {
		*destination[0].(*string) = "webhook-1"
		*destination[1].(*string) = "Partner endpoint"
		*destination[2].(*string) = "https://partner.example.test/hooks"
		*destination[3].(*[]byte) = []byte(`["account.created","transfer.posted"]`)
		*destination[4].(*string) = "kms/webhooks/primary"
		*destination[5].(*string) = "key-001"
		*destination[6].(*string) = "active"
		*destination[7].(*string) = "2"
		return nil
	}))
	if err != nil {
		t.Fatal(err)
	}
	if len(item.SubscribedEvents) != 2 || item.SubscribedEvents[1] != "transfer.posted" {
		t.Fatalf("subscriptions=%#v", item.SubscribedEvents)
	}
}

func TestScanWebhookRejectsMalformedDatabaseArrayProjection(t *testing.T) {
	_, err := scanWebhook(webhookScanFunc(func(destination ...any) error {
		*destination[3].(*[]byte) = []byte(`not-json`)
		return nil
	}))
	if err == nil || err.Error() != "invalid persisted webhook subscriptions" {
		t.Fatalf("error=%v", err)
	}
}

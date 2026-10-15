package webhookdelivery

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type staticResolver struct{ key []byte }

func (r staticResolver) Resolve(context.Context, string) ([]byte, error) { return r.key, nil }

type resolverFunc func(context.Context, string) ([]byte, error)

func (fn resolverFunc) Resolve(ctx context.Context, reference string) ([]byte, error) {
	return fn(ctx, reference)
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) { return fn(request) }

func TestDispatcherSignsBoundedWebhookRequest(t *testing.T) {
	now := time.Date(2026, 8, 28, 14, 0, 0, 0, time.UTC)
	key := []byte(strings.Repeat("k", 32))
	client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.Method != http.MethodPost || request.Header.Get("X-LedgerSync-Event-Type") != "transfer.posted" || request.Header.Get("X-LedgerSync-Key-ID") != "key-001" {
			t.Fatalf("headers=%v method=%s", request.Header, request.Method)
		}
		body, _ := io.ReadAll(request.Body)
		mac := hmac.New(sha256.New, key)
		_, _ = mac.Write([]byte(now.Format(time.RFC3339Nano) + ".delivery-1." + string(body) + ".key-001"))
		if got, want := request.Header.Get("X-LedgerSync-Signature"), "v1="+hex.EncodeToString(mac.Sum(nil)); got != want {
			t.Fatalf("signature=%q want=%q", got, want)
		}
		return &http.Response{StatusCode: http.StatusAccepted, Body: io.NopCloser(strings.NewReader("ok")), Header: make(http.Header)}, nil
	})}
	dispatcher, err := NewDispatcher(client, staticResolver{key: key}, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	outcome, err := dispatcher.Dispatch(context.Background(), Delivery{ID: "delivery-1", EventID: "event-1", EventType: "transfer.posted", EndpointURL: "https://partner.example.test/hooks", SigningKeyReference: "kms/webhooks/primary", SigningKeyID: "key-001", Payload: []byte(`{"transfer_id":"transfer-1"}`), OccurredAt: now})
	if err != nil || outcome.ResponseClass != "http_202" || outcome.Retryable {
		t.Fatalf("outcome=%+v err=%v", outcome, err)
	}
}

func TestValidateDeliveryRejectsPrivateOrInsecureEndpoint(t *testing.T) {
	base := Delivery{ID: "delivery-1", EventID: "event-1", EventType: "transfer.posted", SigningKeyReference: "kms/webhooks/primary", SigningKeyID: "key-001", Payload: []byte(`{}`)}
	for _, endpoint := range []string{"http://partner.example.test/hooks", "https://127.0.0.1/hooks", "https://partner.example.test:8443/hooks", "https://user:pass@partner.example.test/hooks"} {
		base.EndpointURL = endpoint
		if err := validateDelivery(base); err == nil {
			t.Fatalf("endpoint %q was accepted", endpoint)
		}
	}
}

func TestCachedKeyResolverEvictsLeastRecentlyUsedEntryAtCapacity(t *testing.T) {
	var calls sync.Map
	upstream := resolverFunc(func(_ context.Context, reference string) ([]byte, error) {
		counter, _ := calls.LoadOrStore(reference, new(atomic.Int64))
		counter.(*atomic.Int64).Add(1)
		return []byte(strings.Repeat(reference, 32))[:32], nil
	})
	resolver, err := NewCachedKeyResolver(upstream, time.Minute, func() time.Time {
		return time.Date(2026, 9, 3, 12, 0, 0, 0, time.UTC)
	})
	if err != nil {
		t.Fatal(err)
	}

	for index := 0; index < 128; index++ {
		if _, err := resolver.Resolve(context.Background(), fmt.Sprintf("key-%03d", index)); err != nil {
			t.Fatalf("prime key %d: %v", index, err)
		}
	}
	if _, err := resolver.Resolve(context.Background(), "key-000"); err != nil {
		t.Fatalf("refresh least-recently-used order: %v", err)
	}
	if _, err := resolver.Resolve(context.Background(), "key-128"); err != nil {
		t.Fatalf("resolve beyond capacity: %v", err)
	}
	if _, err := resolver.Resolve(context.Background(), "key-000"); err != nil {
		t.Fatalf("recent entry should remain cached: %v", err)
	}
	if _, err := resolver.Resolve(context.Background(), "key-001"); err != nil {
		t.Fatalf("evicted entry should be reloaded: %v", err)
	}

	if got := loadCallCount(&calls, "key-000"); got != 1 {
		t.Fatalf("recent key upstream calls=%d want=1", got)
	}
	if got := loadCallCount(&calls, "key-001"); got != 2 {
		t.Fatalf("least-recently-used key upstream calls=%d want=2", got)
	}
}

func TestCachedKeyResolverConcurrentSaturationCompletes(t *testing.T) {
	upstream := resolverFunc(func(_ context.Context, reference string) ([]byte, error) {
		return []byte(strings.Repeat(reference, 32))[:32], nil
	})
	resolver, err := NewCachedKeyResolver(upstream, time.Minute, nil)
	if err != nil {
		t.Fatal(err)
	}

	const resolutions = 256
	errorsByResolution := make(chan error, resolutions)
	var group sync.WaitGroup
	group.Add(resolutions)
	for index := 0; index < resolutions; index++ {
		go func(index int) {
			defer group.Done()
			_, resolveErr := resolver.Resolve(context.Background(), fmt.Sprintf("key-%03d", index))
			errorsByResolution <- resolveErr
		}(index)
	}
	done := make(chan struct{})
	go func() {
		group.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("concurrent saturated cache did not make progress")
	}
	close(errorsByResolution)
	for err := range errorsByResolution {
		if err != nil {
			t.Fatalf("concurrent resolution failed: %v", err)
		}
	}
}

func TestCachedKeyResolverExpiresAndReloadsKey(t *testing.T) {
	now := time.Date(2026, 9, 3, 12, 0, 0, 0, time.UTC)
	var calls atomic.Int64
	resolver, err := NewCachedKeyResolver(resolverFunc(func(context.Context, string) ([]byte, error) {
		calls.Add(1)
		return []byte(strings.Repeat("k", 32)), nil
	}), time.Minute, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	if _, err := resolver.Resolve(context.Background(), "key"); err != nil {
		t.Fatal(err)
	}
	now = now.Add(time.Minute + time.Nanosecond)
	if _, err := resolver.Resolve(context.Background(), "key"); err != nil {
		t.Fatal(err)
	}
	if got := calls.Load(); got != 2 {
		t.Fatalf("upstream calls=%d want=2", got)
	}
}

func TestCachedKeyResolverDoesNotCacheUpstreamError(t *testing.T) {
	var calls atomic.Int64
	resolver, err := NewCachedKeyResolver(resolverFunc(func(context.Context, string) ([]byte, error) {
		if calls.Add(1) == 1 {
			return nil, errors.New("kms unavailable")
		}
		return []byte(strings.Repeat("k", 32)), nil
	}), time.Minute, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := resolver.Resolve(context.Background(), "key"); err == nil {
		t.Fatal("expected first upstream failure")
	}
	if _, err := resolver.Resolve(context.Background(), "key"); err != nil {
		t.Fatalf("second resolution should retry upstream: %v", err)
	}
	if got := calls.Load(); got != 2 {
		t.Fatalf("upstream calls=%d want=2", got)
	}
}

func TestCachedKeyResolverHonorsCancellationAndRecovers(t *testing.T) {
	var calls atomic.Int64
	resolver, err := NewCachedKeyResolver(resolverFunc(func(ctx context.Context, _ string) ([]byte, error) {
		if calls.Add(1) == 1 {
			<-ctx.Done()
			return nil, ctx.Err()
		}
		return []byte(strings.Repeat("k", 32)), nil
	}), time.Minute, nil)
	if err != nil {
		t.Fatal(err)
	}
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := resolver.Resolve(cancelled, "key"); err == nil {
		t.Fatal("expected cancelled upstream resolution to fail")
	}
	if _, err := resolver.Resolve(context.Background(), "key"); err != nil {
		t.Fatalf("cache remained unavailable after cancellation: %v", err)
	}
}

func loadCallCount(calls *sync.Map, reference string) int64 {
	value, ok := calls.Load(reference)
	if !ok {
		return 0
	}
	return value.(*atomic.Int64).Load()
}

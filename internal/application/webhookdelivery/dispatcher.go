// Package webhookdelivery builds bounded, signed webhook requests. Queueing and
// immutable attempt evidence remain separate platform responsibilities.
package webhookdelivery

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"time"
)

const maxPayloadBytes = 256 * 1024

type KeyResolver interface {
	Resolve(context.Context, string) ([]byte, error)
}

type Delivery struct {
	ID, EventID, EventType, EndpointURL, SigningKeyReference, SigningKeyID string
	Payload                                                                json.RawMessage
	OccurredAt                                                             time.Time
}

type Outcome struct {
	ResponseClass string
	Retryable     bool
}

type Dispatcher struct {
	client   *http.Client
	resolver KeyResolver
	clock    func() time.Time
}

func NewDispatcher(client *http.Client, resolver KeyResolver, clock func() time.Time) (*Dispatcher, error) {
	if client == nil || resolver == nil {
		return nil, errors.New("webhook HTTP client and signing key resolver are required")
	}
	if clock == nil {
		clock = time.Now
	}
	return &Dispatcher{client: client, resolver: resolver, clock: clock}, nil
}

func (d *Dispatcher) Dispatch(ctx context.Context, delivery Delivery) (Outcome, error) {
	if err := validateDelivery(delivery); err != nil {
		return Outcome{}, err
	}
	key, err := d.resolver.Resolve(ctx, delivery.SigningKeyReference)
	if err != nil || len(key) < 32 {
		return Outcome{ResponseClass: "key_unavailable", Retryable: true}, errors.New("webhook signing key is unavailable")
	}
	key = bytes.Clone(key)
	defer clear(key)
	stamp := d.clock().UTC().Format(time.RFC3339Nano)
	message := append(append(append([]byte(stamp+"."+delivery.ID+"."), delivery.Payload...), '.'), []byte(delivery.SigningKeyID)...)
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write(message)
	signature := "v1=" + hex.EncodeToString(mac.Sum(nil))

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, delivery.EndpointURL, bytes.NewReader(delivery.Payload))
	if err != nil {
		return Outcome{}, fmt.Errorf("build webhook request: %w", err)
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("User-Agent", "LedgerSync-Webhooks/1")
	request.Header.Set("X-LedgerSync-Delivery-ID", delivery.ID)
	request.Header.Set("X-LedgerSync-Event-ID", delivery.EventID)
	request.Header.Set("X-LedgerSync-Event-Type", delivery.EventType)
	request.Header.Set("X-LedgerSync-Timestamp", stamp)
	request.Header.Set("X-LedgerSync-Key-ID", delivery.SigningKeyID)
	request.Header.Set("X-LedgerSync-Signature", signature)
	response, err := d.client.Do(request)
	if err != nil {
		return Outcome{ResponseClass: "network_error", Retryable: true}, fmt.Errorf("send webhook: %w", err)
	}
	defer func() { _, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 8*1024)); _ = response.Body.Close() }()
	class := fmt.Sprintf("http_%d", response.StatusCode)
	if response.StatusCode >= 200 && response.StatusCode < 300 {
		return Outcome{ResponseClass: class}, nil
	}
	return Outcome{ResponseClass: class, Retryable: response.StatusCode == http.StatusRequestTimeout || response.StatusCode == http.StatusTooManyRequests || response.StatusCode >= 500}, errors.New("webhook endpoint rejected delivery")
}

func validateDelivery(delivery Delivery) error {
	if strings.TrimSpace(delivery.ID) == "" || strings.TrimSpace(delivery.EventID) == "" || strings.TrimSpace(delivery.EventType) == "" || strings.TrimSpace(delivery.SigningKeyReference) == "" || strings.TrimSpace(delivery.SigningKeyID) == "" || len(delivery.Payload) == 0 || len(delivery.Payload) > maxPayloadBytes || !json.Valid(delivery.Payload) {
		return errors.New("complete bounded webhook delivery is required")
	}
	endpoint, err := url.Parse(delivery.EndpointURL)
	if err != nil || endpoint.Scheme != "https" || endpoint.Hostname() == "" || endpoint.User != nil || endpoint.Port() != "" && endpoint.Port() != "443" {
		return errors.New("webhook endpoint must be credential-free HTTPS on port 443")
	}
	if address, err := netip.ParseAddr(endpoint.Hostname()); err == nil && (address.IsPrivate() || address.IsLoopback() || address.IsLinkLocalUnicast() || address.IsMulticast() || address.IsUnspecified()) {
		return errors.New("webhook endpoint resolves to a non-public address")
	}
	return nil
}

func clear(value []byte) {
	for index := range value {
		value[index] = 0
	}
}

// PublicDialContext resolves each outbound connection and rejects private
// addresses, including a hostname that changes after endpoint verification.
func PublicDialContext(resolver *net.Resolver, dialer *net.Dialer) func(context.Context, string, string) (net.Conn, error) {
	if resolver == nil {
		resolver = net.DefaultResolver
	}
	if dialer == nil {
		dialer = &net.Dialer{Timeout: 5 * time.Second, KeepAlive: 30 * time.Second}
	}
	return func(ctx context.Context, network, address string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(address)
		if err != nil {
			return nil, err
		}
		addresses, err := resolver.LookupNetIP(ctx, "ip", host)
		if err != nil {
			return nil, err
		}
		for _, address := range addresses {
			if address.IsPrivate() || address.IsLoopback() || address.IsLinkLocalUnicast() || address.IsMulticast() || address.IsUnspecified() {
				continue
			}
			return dialer.DialContext(ctx, network, net.JoinHostPort(address.String(), port))
		}
		return nil, errors.New("webhook hostname has no public address")
	}
}

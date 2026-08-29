package webhookverification

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
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/webhookdelivery"
)

const verificationProofHeader = "X-LedgerSync-Verification"

type Dispatcher struct {
	client   *http.Client
	resolver webhookdelivery.KeyResolver
	clock    func() time.Time
}

func NewDispatcher(client *http.Client, resolver webhookdelivery.KeyResolver, clock func() time.Time) (*Dispatcher, error) {
	if client == nil || resolver == nil {
		return nil, errors.New("webhook verification HTTP client and signing key resolver are required")
	}
	if clock == nil {
		clock = time.Now
	}
	return &Dispatcher{client: client, resolver: resolver, clock: clock}, nil
}

func (d *Dispatcher) Verify(ctx context.Context, job Job) (Outcome, error) {
	if strings.TrimSpace(job.ID) == "" || strings.TrimSpace(job.EndpointURL) == "" || strings.TrimSpace(job.SigningKeyReference) == "" || strings.TrimSpace(job.SigningKeyID) == "" || len(job.Challenge) < 32 || len(job.Challenge) > 255 {
		return Outcome{ResponseClass: "invalid_job"}, errors.New("incomplete webhook verification job")
	}
	endpoint, err := url.Parse(job.EndpointURL)
	if err != nil || endpoint.Scheme != "https" || endpoint.Hostname() == "" || endpoint.User != nil {
		return Outcome{ResponseClass: "invalid_endpoint"}, errors.New("webhook verification endpoint must be credential-free HTTPS")
	}
	key, err := d.resolver.Resolve(ctx, job.SigningKeyReference)
	if err != nil || len(key) < 32 {
		return Outcome{ResponseClass: "key_unavailable", Retryable: true}, errors.New("webhook verification signing key is unavailable")
	}
	key = bytes.Clone(key)
	defer clear(key)
	challenge := string(job.Challenge)
	body, err := json.Marshal(struct {
		Challenge string `json:"challenge"`
		ExpiresAt string `json:"expires_at"`
	}{Challenge: challenge, ExpiresAt: job.ExpiresAt.UTC().Format(time.RFC3339Nano)})
	if err != nil {
		return Outcome{}, err
	}
	stamp := d.clock().UTC().Format(time.RFC3339Nano)
	signature := hmacHex(key, stamp+"."+job.ID+"."+string(body)+"."+job.SigningKeyID)
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, job.EndpointURL, bytes.NewReader(body))
	if err != nil {
		return Outcome{ResponseClass: "request_invalid"}, err
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("User-Agent", "LedgerSync-Webhook-Verification/1")
	request.Header.Set("X-LedgerSync-Verification-ID", job.ID)
	request.Header.Set("X-LedgerSync-Timestamp", stamp)
	request.Header.Set("X-LedgerSync-Key-ID", job.SigningKeyID)
	request.Header.Set("X-LedgerSync-Signature", "v1="+signature)
	response, err := d.client.Do(request)
	if err != nil {
		return Outcome{ResponseClass: "network_error", Retryable: true}, fmt.Errorf("send webhook verification: %w", err)
	}
	defer func() { _, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 8*1024)); _ = response.Body.Close() }()
	class := fmt.Sprintf("http_%d", response.StatusCode)
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return Outcome{ResponseClass: class, Retryable: response.StatusCode == http.StatusRequestTimeout || response.StatusCode == http.StatusTooManyRequests || response.StatusCode >= 500}, errors.New("webhook endpoint rejected verification")
	}
	proof := strings.TrimPrefix(strings.TrimSpace(response.Header.Get(verificationProofHeader)), "v1=")
	if proof == "" || !hmac.Equal([]byte(proof), []byte(hmacHex(key, challenge))) {
		return Outcome{ResponseClass: "invalid_proof"}, errors.New("webhook endpoint did not prove control")
	}
	return Outcome{ResponseClass: class}, nil
}

func hmacHex(key []byte, value string) string {
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write([]byte(value))
	return hex.EncodeToString(mac.Sum(nil))
}

func clear(value []byte) {
	for index := range value {
		value[index] = 0
	}
}

package config

import (
	"encoding/base64"
	"testing"
)

func TestParseWebhookSigningKeysAcceptsBoundedBase64Values(t *testing.T) {
	raw := make([]byte, 32)
	for index := range raw {
		raw[index] = byte(index + 1)
	}
	keys, err := parseWebhookSigningKeys(`{"secrets/webhooks/partner":` + quote(base64.RawStdEncoding.EncodeToString(raw)) + `}`)
	if err != nil {
		t.Fatal(err)
	}
	if got := keys["secrets/webhooks/partner"]; string(got) != string(raw) {
		t.Fatalf("parsed key=%x", got)
	}
}

func TestParseWebhookSigningKeysRejectsShortOrMalformedValues(t *testing.T) {
	for _, value := range []string{`{"secrets/webhooks/partner":"not-base64"}`, `{"secrets/webhooks/partner":"c2hvcnQ="}`, `[]`} {
		if _, err := parseWebhookSigningKeys(value); err == nil {
			t.Fatalf("parseWebhookSigningKeys(%s) unexpectedly succeeded", value)
		}
	}
}

func quote(value string) string { return `"` + value + `"` }

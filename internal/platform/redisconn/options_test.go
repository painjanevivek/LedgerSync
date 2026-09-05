package redisconn

import (
	"crypto/tls"
	"testing"
)

func TestOptionsAcceptsLocalAddress(t *testing.T) {
	options, err := Options("redis:6379")
	if err != nil {
		t.Fatal(err)
	}
	if options.Addr != "redis:6379" || options.TLSConfig != nil {
		t.Fatalf("unexpected local options: %#v", options)
	}
}

func TestOptionsParsesManagedTLSURL(t *testing.T) {
	options, err := Options("rediss://default:secret@cache.example:6380/2")
	if err != nil {
		t.Fatal(err)
	}
	if options.Addr != "cache.example:6380" || options.Username != "default" || options.Password != "secret" || options.DB != 2 {
		t.Fatalf("managed URL fields were not parsed: %#v", options)
	}
	if options.TLSConfig == nil || options.TLSConfig.MinVersion != tls.VersionTLS12 {
		t.Fatalf("expected TLS options for rediss URL: %#v", options.TLSConfig)
	}
}

func TestOptionsRejectsEmptyAddress(t *testing.T) {
	if _, err := Options(" "); err == nil {
		t.Fatal("expected an empty Redis address to fail")
	}
}

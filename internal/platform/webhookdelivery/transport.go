package webhookdelivery

import (
	"context"
	"crypto/tls"
	"errors"
	"net"
	"net/http"
	"time"
)

// NewSecureHTTPClient fixes the request boundary for webhook delivery: short
// bounded connections, no redirects, modern TLS, and DNS-rebinding-resistant
// public-address dialing.
func NewSecureHTTPClient() *http.Client {
	transport := &http.Transport{
		DialContext:           PublicDialContext(nil, nil),
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          32,
		MaxIdleConnsPerHost:   4,
		IdleConnTimeout:       30 * time.Second,
		TLSHandshakeTimeout:   5 * time.Second,
		ResponseHeaderTimeout: 5 * time.Second,
		ExpectContinueTimeout: time.Second,
		TLSClientConfig:       &tls.Config{MinVersion: tls.VersionTLS12},
	}
	return &http.Client{
		Transport: transport,
		Timeout:   10 * time.Second,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
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

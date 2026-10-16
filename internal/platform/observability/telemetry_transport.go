package observability

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
)

// telemetryTransport accepts an authority only. Credentials, paths and query
// strings cannot enter an exporter target. Bare host:port is local compatibility.
func telemetryTransport(endpoint, caFile string) (string, *tls.Config, error) {
	if endpoint == "" || len(endpoint) > 256 || strings.TrimSpace(endpoint) != endpoint {
		return "", nil, errors.New("invalid private telemetry endpoint")
	}
	explicit := strings.Contains(endpoint, "://")
	if !explicit {
		endpoint = "http://" + endpoint
	}
	parsed, err := url.Parse(endpoint)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.User != nil || (parsed.Path != "" && parsed.Path != "/") || parsed.RawQuery != "" || parsed.Fragment != "" || parsed.Opaque != "" {
		return "", nil, errors.New("invalid private telemetry authority")
	}
	if !explicit {
		host, port, err := net.SplitHostPort(parsed.Host)
		if err != nil || host == "" || port == "" {
			return "", nil, errors.New("private telemetry host and port required")
		}
	}
	if parsed.Hostname() == "" || strings.HasSuffix(parsed.Host, ":") {
		return "", nil, errors.New("invalid telemetry host")
	}
	if port := parsed.Port(); port != "" {
		number, err := strconv.Atoi(port)
		if err != nil || number < 1 || number > 65535 {
			return "", nil, errors.New("invalid telemetry port")
		}
	}
	parsed.Path = ""
	if parsed.Scheme == "http" {
		if caFile != "" {
			return "", nil, errors.New("telemetry CA requires HTTPS")
		}
		return parsed.String(), nil, nil
	}
	config := &tls.Config{MinVersion: tls.VersionTLS12}
	if caFile != "" {
		pem, err := os.ReadFile(caFile)
		if err != nil {
			return "", nil, errors.New("telemetry CA file unavailable")
		}
		roots, err := x509.SystemCertPool()
		if err != nil {
			roots = x509.NewCertPool()
		}
		if !roots.AppendCertsFromPEM(pem) {
			return "", nil, errors.New("invalid telemetry CA")
		}
		config.RootCAs = roots
	}
	return parsed.String(), config, nil
}

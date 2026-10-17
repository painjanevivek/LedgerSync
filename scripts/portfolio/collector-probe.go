//go:build ignore

// collector-probe is a local synthetic TLS receiver. It is not an app service.
package main

import (
	"compress/gzip"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"flag"
	"io"
	"math/big"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

func main() {
	root := flag.String("root", "", "exact private collector evidence directory")
	flag.Parse()
	absolute, err := filepath.Abs(*root)
	if err != nil {
		panic(err)
	}
	cwd, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	relative, err := filepath.Rel(filepath.Join(cwd, "data", "portfolio"), absolute)
	if err != nil || !regexp.MustCompile(`^r17-\d{8}[/\\]collector-\d{14}-[a-f0-9]{8}$`).MatchString(relative) {
		panic("private collector evidence path required")
	}
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		panic(err)
	}
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		panic(err)
	}
	certificate := &x509.Certificate{SerialNumber: serial, Subject: pkix.Name{CommonName: "LedgerSync local synthetic collector probe"}, NotBefore: time.Now().Add(-time.Minute), NotAfter: time.Now().Add(2 * time.Hour), IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}, DNSNames: []string{"host.docker.internal", "localhost"}, IPAddresses: []net.IP{net.ParseIP("127.0.0.1")}}
	der, err := x509.CreateCertificate(rand.Reader, certificate, certificate, &key.PublicKey, key)
	if err != nil {
		panic(err)
	}
	encoded := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyDER, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		panic(err)
	}
	write := func(name string, value []byte) {
		if err := os.WriteFile(filepath.Join(absolute, name), value, 0600); err != nil {
			panic(err)
		}
	}
	write("ca.crt", encoded)
	write("receiver.crt", encoded)
	write("receiver.key", pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER}))
	pair, err := tls.X509KeyPair(encoded, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER}))
	if err != nil {
		panic(err)
	}
	listener, err := tls.Listen("tcp", "127.0.0.1:0", &tls.Config{Certificates: []tls.Certificate{pair}, MinVersion: tls.VersionTLS12})
	if err != nil {
		panic(err)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /v1/traces", func(w http.ResponseWriter, r *http.Request) {
		if r.TLS == nil || r.TLS.Version < tls.VersionTLS12 {
			http.Error(w, "TLS required", 400)
			return
		}
		var body io.Reader = http.MaxBytesReader(w, r.Body, 1<<20)
		if r.Header.Get("Content-Encoding") == "gzip" {
			reader, e := gzip.NewReader(body)
			if e != nil {
				http.Error(w, "invalid compression", 400)
				return
			}
			defer reader.Close()
			body = io.LimitReader(reader, (1<<20)+1)
		}
		raw, e := io.ReadAll(body)
		if e != nil || len(raw) > 1<<20 || !json.Valid(raw) {
			http.Error(w, "invalid bounded JSON", 400)
			return
		}
		result, _ := json.Marshal(map[string]any{"tls_verified_transport": true, "tls_version": r.TLS.Version, "payload": json.RawMessage(raw)})
		write("received.json", result)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{}`))
	})
	server := &http.Server{Handler: mux, ReadHeaderTimeout: 3 * time.Second, ReadTimeout: 3 * time.Second, WriteTimeout: 3 * time.Second, MaxHeaderBytes: 8192}
	port := strings.Split(listener.Addr().String(), ":")
	ready, _ := json.Marshal(map[string]string{"endpoint": "https://host.docker.internal:" + port[len(port)-1]})
	write("ready.json", ready)
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	go func() {
		<-ctx.Done()
		shutdown, stop := context.WithTimeout(context.Background(), 3*time.Second)
		defer stop()
		_ = server.Shutdown(shutdown)
	}()
	if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
		panic(err)
	}
}

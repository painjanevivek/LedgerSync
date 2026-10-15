package routing

import (
	"bufio"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestStudyProxyDiscoversReplacementsAndPreservesRequests(t *testing.T) {
	var ready atomic.Bool
	ready.Store(true)
	var calls atomic.Int64
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/readyz" {
			if !ready.Load() {
				w.WriteHeader(503)
				return
			}
			w.Header().Set("X-LedgerSync-Node", "api-a")
			return
		}
		calls.Add(1)
		if r.Method != "POST" || r.Header.Get("X-LedgerSync-Actor-Assertion") != "signed" || r.Header.Get("X-Request-ID") != "request" || r.Header.Get("X-Correlation-ID") != "correlation" {
			t.Error("request metadata was changed")
		}
		body, _ := io.ReadAll(r.Body)
		if string(body) != "exact" {
			t.Error("body was changed")
		}
		w.WriteHeader(201)
	}))
	defer upstream.Close()
	addresses := []string{upstream.URL}
	proxy := NewStudyProxy(func(context.Context) ([]string, error) { return addresses, nil })
	proxy.Refresh(context.Background())
	request := httptest.NewRequest("POST", "http://gateway/api/transfers", strings.NewReader("exact"))
	request.Header.Set("X-LedgerSync-Actor-Assertion", "signed")
	request.Header.Set("X-Request-ID", "request")
	request.Header.Set("X-Correlation-ID", "correlation")
	response := httptest.NewRecorder()
	proxy.ServeHTTP(response, request)
	if response.Code != 201 || calls.Load() != 1 {
		t.Fatal("command was not delivered exactly once by the gateway")
	}
	ready.Store(false)
	proxy.Refresh(context.Background())
	response = httptest.NewRecorder()
	proxy.ServeHTTP(response, request)
	if response.Code != 503 || calls.Load() != 1 {
		t.Fatal("unready API received a command")
	}
	replacement := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Header().Set("X-LedgerSync-Node", "api-b") }))
	defer replacement.Close()
	addresses = []string{replacement.URL}
	proxy.Refresh(context.Background())
	snapshot := proxy.Snapshot()
	if snapshot.ReadyAPICount != 1 || snapshot.Nodes[0] != "api-b" {
		t.Fatal("replacement was not discovered")
	}
}

func TestStudyProxyBalancesAndFlushesStreams(t *testing.T) {
	makeServer := func(node string) *httptest.Server {
		return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-LedgerSync-Node", node)
			if r.URL.Path == "/readyz" {
				return
			}
			w.Header().Set("Content-Type", "text/event-stream")
			_, _ = w.Write([]byte("data: " + node + "\n\n"))
			w.(http.Flusher).Flush()
			<-r.Context().Done()
		}))
	}
	a, b := makeServer("a"), makeServer("b")
	defer a.Close()
	defer b.Close()
	proxy := NewStudyProxy(func(context.Context) ([]string, error) { return []string{a.URL, b.URL}, nil })
	proxy.Refresh(context.Background())
	gateway := httptest.NewServer(proxy)
	defer gateway.Close()
	seen := map[string]bool{}
	for i := 0; i < 2; i++ {
		ctx, cancel := context.WithCancel(context.Background())
		req, _ := http.NewRequestWithContext(ctx, "GET", gateway.URL+"/stream", nil)
		response, err := http.DefaultClient.Do(req)
		if err != nil {
			cancel()
			t.Fatal(err)
		}
		line, err := bufio.NewReader(response.Body).ReadString('\n')
		if err != nil {
			t.Fatal(err)
		}
		seen[line] = true
		cancel()
		_ = response.Body.Close()
	}
	if len(seen) != 2 {
		t.Fatal("traffic did not reach both APIs")
	}
}

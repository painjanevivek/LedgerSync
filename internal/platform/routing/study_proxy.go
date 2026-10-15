// Package routing contains the private study gateway. It uses readiness checks
// and Docker DNS to route to current API instances. It never retries a command.
package routing

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

type Endpoint struct{ URL, Node string }
type Snapshot struct {
	CapturedAt    time.Time `json:"captured_at"`
	ReadyAPICount int       `json:"ready_api_count"`
	Nodes         []string  `json:"nodes"`
}
type Resolver func(context.Context) ([]string, error)
type StudyProxy struct {
	resolve    Resolver
	client     *http.Client
	mu         sync.RWMutex
	endpoints  []Endpoint
	capturedAt time.Time
	sequence   atomic.Uint64
	proxy      *httputil.ReverseProxy
}
type routeKey struct{}

func NewStudyProxy(resolve Resolver) *StudyProxy {
	transport := &http.Transport{DialContext: (&net.Dialer{Timeout: time.Second, KeepAlive: 30 * time.Second}).DialContext,
		MaxIdleConns: 64, MaxIdleConnsPerHost: 16, IdleConnTimeout: 60 * time.Second, ResponseHeaderTimeout: 5 * time.Second}
	p := &StudyProxy{resolve: resolve, client: &http.Client{Timeout: time.Second, Transport: transport}}
	p.proxy = &httputil.ReverseProxy{
		Transport: transport, FlushInterval: -1,
		Rewrite: func(r *httputil.ProxyRequest) {
			r.SetURL(r.In.Context().Value(routeKey{}).(*url.URL))
			r.SetXForwarded()
		},
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, _ error) {
			target := r.Context().Value(routeKey{}).(*url.URL)
			p.remove(target.String())
			unavailable(w)
		},
	}
	return p
}

func DockerAPIResolver(ctx context.Context) ([]string, error) {
	addresses, err := net.DefaultResolver.LookupHost(ctx, "api")
	if err != nil {
		return nil, err
	}
	sort.Strings(addresses)
	result := make([]string, 0, len(addresses))
	for _, address := range addresses {
		result = append(result, "http://"+net.JoinHostPort(address, "8080"))
	}
	return result, nil
}

// Refresh replaces the entire set. A failed DNS or readiness observation does
// not leave an old endpoint available indefinitely.
func (p *StudyProxy) Refresh(ctx context.Context) {
	addresses, err := p.resolve(ctx)
	ready := []Endpoint{}
	if err == nil {
		if len(addresses) > 16 {
			addresses = addresses[:16]
		}
		var wg sync.WaitGroup
		var mu sync.Mutex
		for _, address := range addresses {
			wg.Add(1)
			go func(address string) {
				defer wg.Done()
				request, err := http.NewRequestWithContext(ctx, http.MethodGet, address+"/readyz", nil)
				if err != nil {
					return
				}
				response, err := p.client.Do(request)
				if err != nil {
					return
				}
				_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 1024))
				_ = response.Body.Close()
				if response.StatusCode != http.StatusOK {
					return
				}
				mu.Lock()
				ready = append(ready, Endpoint{URL: address, Node: response.Header.Get("X-LedgerSync-Node")})
				mu.Unlock()
			}(address)
		}
		wg.Wait()
	}
	sort.Slice(ready, func(i, j int) bool { return ready[i].URL < ready[j].URL })
	p.mu.Lock()
	p.endpoints = ready
	p.capturedAt = time.Now().UTC()
	p.mu.Unlock()
}

func (p *StudyProxy) Run(ctx context.Context) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			p.Refresh(ctx)
		}
	}
}

func (p *StudyProxy) Snapshot() Snapshot {
	p.mu.RLock()
	defer p.mu.RUnlock()
	nodes := make([]string, 0, len(p.endpoints))
	for _, endpoint := range p.endpoints {
		if endpoint.Node != "" {
			nodes = append(nodes, endpoint.Node)
		}
	}
	return Snapshot{CapturedAt: p.capturedAt, ReadyAPICount: len(p.endpoints), Nodes: nodes}
}
func (p *StudyProxy) remove(address string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	retained := p.endpoints[:0]
	for _, endpoint := range p.endpoints {
		if endpoint.URL != address {
			retained = append(retained, endpoint)
		}
	}
	p.endpoints = retained
}
func unavailable(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Retry-After", "1")
	w.WriteHeader(http.StatusServiceUnavailable)
	_, _ = w.Write([]byte(`{"error":{"code":"temporary_unavailable"}}`))
}
func (p *StudyProxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/internal/study/replicas" && r.Method == http.MethodGet {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		_ = json.NewEncoder(w).Encode(p.Snapshot())
		return
	}
	if r.URL.Path == "/readyz" && r.Method == http.MethodGet {
		if p.Snapshot().ReadyAPICount == 0 {
			unavailable(w)
			return
		}
		w.WriteHeader(http.StatusOK)
		return
	}
	p.mu.RLock()
	if len(p.endpoints) == 0 {
		p.mu.RUnlock()
		unavailable(w)
		return
	}
	endpoint := p.endpoints[(p.sequence.Add(1)-1)%uint64(len(p.endpoints))]
	p.mu.RUnlock()
	target, _ := url.Parse(endpoint.URL)
	p.proxy.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), routeKey{}, target)))
}

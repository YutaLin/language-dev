package main

import (
	"context"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"sync"
	"sync/atomic"
	"time"
)

type ctxKey int

const attemptsKey ctxKey = iota
const maxAttempts = 3

type Backend struct {
	URL   *url.URL
	Proxy *httputil.ReverseProxy

	mu    sync.RWMutex
	alive bool
}

func (b *Backend) SetAlive(alive bool) {
	b.mu.Lock()
	b.alive = alive
	b.mu.Unlock()
}

func (b *Backend) SetAliveIfChanged(alive bool) (changed bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.alive != alive {
		b.alive = alive
		return true
	}
	return false
}

func (b *Backend) IsAlive() bool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.alive
}

type LoadBalancer struct {
	backends []*Backend
	current  atomic.Uint64
}

func NewLoadBalancer(targets []string) (*LoadBalancer, error) {
	var backends []*Backend
	for _, t := range targets {
		u, err := url.Parse(t)
		if err != nil {
			return nil, err
		}
		backends = append(backends, &Backend{
			URL:   u,
			Proxy: httputil.NewSingleHostReverseProxy(u),
			alive: true,
		})
	}

	lb := &LoadBalancer{backends: backends}

	for _, b := range backends {
		b := b
		b.Proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
			if b.SetAliveIfChanged(false) {
				log.Printf("backend %s marked dead (passive): %v", b.URL, err)
			}
			lb.retry(w, r)
		}
	}

	return lb, nil
}

func (lb *LoadBalancer) retry(w http.ResponseWriter, r *http.Request) {
	attempts := attemptsFromContext(r)
	if attempts >= maxAttempts {
		http.Error(w, "no healthy backends", http.StatusServiceUnavailable)
		return
	}

	b := lb.next()
	if b == nil {
		http.Error(w, "no healthy backends", http.StatusServiceUnavailable)
		return
	}

	ctx := context.WithValue(r.Context(), attemptsKey, attempts+1)
	log.Printf("retry #%d routing %s -> %s", attempts+1, r.URL.Path, b.URL)
	b.Proxy.ServeHTTP(w, r.WithContext(ctx))
}

func attemptsFromContext(r *http.Request) int {
	if v, ok := r.Context().Value(attemptsKey).(int); ok {
		return v
	}
	return 0
}

func (lb *LoadBalancer) next() *Backend {
	n := len(lb.backends)
	start := lb.current.Add(1) - 1
	for i := 0; i < n; i++ {
		idx := (start + uint64(i)) % uint64(n)
		b := lb.backends[idx]
		if b.IsAlive() {
			return b
		}
	}
	return nil
}

func (lb *LoadBalancer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	r = r.WithContext(ctx)

	b := lb.next()
	if b == nil {
		http.Error(w, "no healthy backends", http.StatusServiceUnavailable)
		return
	}
	log.Printf("routing %s -> %s", r.URL.Path, b.URL)
	b.Proxy.ServeHTTP(w, r)
}

func (lb *LoadBalancer) checkHealth() {
	for _, b := range lb.backends {
		alive := isBackendAlive(b.URL)
		if b.SetAliveIfChanged(alive) {
			log.Printf("backend %s changed (active): alive=%v", b.URL, alive)
		}
	}
}

func isBackendAlive(u *url.URL) bool {
	client := http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(u.String() + "/healthz")
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

// HealthLoop runs forever, checking every interval. Run it in its own goroutine.
func (lb *LoadBalancer) HealthLoop(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for range ticker.C {
		lb.checkHealth()
	}
}

func main() {
	targets := []string{
		"http://localhost:9001",
		"http://localhost:9002",
		"http://localhost:9003",
	}

	lb, err := NewLoadBalancer(targets)
	if err != nil {
		log.Fatal(err)
	}

	go lb.HealthLoop(2 * time.Second)

	log.Println("load balancer listening on :8080")
	if err := http.ListenAndServe(":8080", lb); err != nil {
		log.Fatal(err)
	}
}

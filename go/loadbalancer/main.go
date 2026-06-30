package main

import (
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"sync"
	"sync/atomic"
	"time"
)

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

	return &LoadBalancer{backends: backends}, nil
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
		was := b.IsAlive()
		b.SetAlive(alive)
		if was != alive {
			log.Printf("backend %s changed: alive=%v", b.URL, alive)
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

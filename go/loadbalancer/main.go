package main

import (
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"sync/atomic"
)

type Backend struct {
	URL   *url.URL
	Proxy *httputil.ReverseProxy
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
		})
	}

	return &LoadBalancer{backends: backends}, nil
}

func (lb *LoadBalancer) next() *Backend {
	n := lb.current.Add(1)
	idx := (n - 1) % uint64(len(lb.backends))
	return lb.backends[idx]
}

func (lb *LoadBalancer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	b := lb.next()
	log.Printf("routing %s -> %s", r.URL.Path, b.URL)
	b.Proxy.ServeHTTP(w, r)
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

	log.Println("load balancer listening on :8080")
	if err := http.ListenAndServe(":8080", lb); err != nil {
		log.Fatal(err)
	}
}

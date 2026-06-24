package main

import (
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
)

func main() {
	target, err := url.Parse("http://localhost:9001")
	if err != nil {
		log.Fatal(err)
	}

	proxy := httputil.NewSingleHostReverseProxy(target)
	log.Println("load balancer listening on :8080")
	if err := http.ListenAndServe(":8080", proxy); err != nil {
		log.Fatal(err)
	}
}

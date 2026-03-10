//go:build ignore

package main

import (
	"flag"
	"fmt"
	"net/http"
)

// Fake app — stands in for a production service.
// Serves a health endpoint that PageFire monitors.
func main() {
	port := flag.Int("port", 8080, "port to listen on")
	flag.Parse()

	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, `{"status":"ok"}`)
	})

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "Welcome to MyApp!")
	})

	addr := fmt.Sprintf(":%d", *port)
	fmt.Printf("MyApp listening on %s\n", addr)
	http.ListenAndServe(addr, mux)
}

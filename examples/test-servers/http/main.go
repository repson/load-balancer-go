package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"sync/atomic"
)

var requestCount atomic.Uint64

func main() {
	// Start three HTTP servers on different ports
	go startServer(3001, "Server-1")
	go startServer(3002, "Server-2")
	go startServer(3003, "Server-3")

	// Block forever
	select {}
}

func startServer(port int, name string) {
	var localCount atomic.Uint64

	mux := http.NewServeMux()
	
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		local := localCount.Add(1)
		global := requestCount.Add(1)
		
		response := fmt.Sprintf("Response from %s (port %d)\n", name, port)
		response += fmt.Sprintf("Local requests: %d\n", local)
		response += fmt.Sprintf("Total requests: %d\n", global)
		response += fmt.Sprintf("Path: %s\n", r.URL.Path)
		response += fmt.Sprintf("Method: %s\n", r.Method)
		
		log.Printf("[%s] Request #%d from %s - Path: %s", name, local, r.RemoteAddr, r.URL.Path)
		
		w.Header().Set("X-Server-Name", name)
		w.Header().Set("X-Server-Port", fmt.Sprintf("%d", port))
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(response))
	})
	
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(fmt.Sprintf("OK - %s", name)))
	})
	
	addr := fmt.Sprintf(":%d", port)
	log.Printf("Starting %s on http://localhost%s", name, addr)
	
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Printf("Error starting %s: %v", name, err)
		os.Exit(1)
	}
}

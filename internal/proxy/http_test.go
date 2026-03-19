package proxy

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/isaac/load-balancer-go/internal/backend"
	"github.com/isaac/load-balancer-go/internal/balancer"
)

func TestNewHTTPProxy(t *testing.T) {
	backends := []*backend.Backend{
		backend.New("http://localhost:3001", "", 1),
	}
	bal := balancer.NewRoundRobin(backends)

	proxy := NewHTTPProxy(bal, 3, 100*time.Millisecond)

	if proxy.balancer == nil {
		t.Error("Expected balancer to be set")
	}

	if proxy.maxRetries != 3 {
		t.Errorf("Expected maxRetries 3, got %d", proxy.maxRetries)
	}

	if proxy.retryDelay != 100*time.Millisecond {
		t.Errorf("Expected retryDelay 100ms, got %v", proxy.retryDelay)
	}
}

func TestHTTPProxy_ServeHTTP_Success(t *testing.T) {
	// Create a test backend server
	backendServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("backend response"))
	}))
	defer backendServer.Close()

	// Create proxy
	backends := []*backend.Backend{
		backend.New(backendServer.URL, "", 1),
	}
	bal := balancer.NewRoundRobin(backends)
	proxy := NewHTTPProxy(bal, 3, 10*time.Millisecond)

	// Create request
	req := httptest.NewRequest("GET", "http://example.com/test", nil)
	w := httptest.NewRecorder()

	// Serve request
	proxy.ServeHTTP(w, req)

	// Verify response
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	if w.Body.String() != "backend response" {
		t.Errorf("Expected 'backend response', got %s", w.Body.String())
	}
}

func TestHTTPProxy_ServeHTTP_NoBackends(t *testing.T) {
	// Create proxy with no backends
	backends := []*backend.Backend{}
	bal := balancer.NewRoundRobin(backends)
	proxy := NewHTTPProxy(bal, 3, 10*time.Millisecond)

	// Create request
	req := httptest.NewRequest("GET", "http://example.com/test", nil)
	w := httptest.NewRecorder()

	// Serve request
	proxy.ServeHTTP(w, req)

	// Verify response
	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status 503, got %d", w.Code)
	}
}

func TestHTTPProxy_ServeHTTP_BackendFailure(t *testing.T) {
	// Create a failing backend server
	backendServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("backend error"))
	}))
	defer backendServer.Close()

	// Create proxy with retries
	backends := []*backend.Backend{
		backend.New(backendServer.URL, "", 1),
	}
	bal := balancer.NewRoundRobin(backends)
	proxy := NewHTTPProxy(bal, 2, 10*time.Millisecond)

	// Create request
	req := httptest.NewRequest("GET", "http://example.com/test", nil)
	w := httptest.NewRecorder()

	// Serve request
	proxy.ServeHTTP(w, req)

	// Should fail after retries
	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status 503, got %d", w.Code)
	}
}

func TestHTTPProxy_ServeHTTP_Retry(t *testing.T) {
	requestCount := 0

	// Create a backend that fails twice then succeeds
	backendServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		if requestCount < 3 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("success"))
	}))
	defer backendServer.Close()

	// Create proxy with retries
	backends := []*backend.Backend{
		backend.New(backendServer.URL, "", 1),
	}
	bal := balancer.NewRoundRobin(backends)
	proxy := NewHTTPProxy(bal, 3, 10*time.Millisecond)

	// Create request
	req := httptest.NewRequest("GET", "http://example.com/test", nil)
	w := httptest.NewRecorder()

	// Serve request
	proxy.ServeHTTP(w, req)

	// Should succeed on third attempt
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	if requestCount != 3 {
		t.Errorf("Expected 3 requests, got %d", requestCount)
	}
}

func TestHTTPProxy_ConnectionCounting(t *testing.T) {
	// Create a backend server
	backendServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer backendServer.Close()

	// Create proxy
	backends := []*backend.Backend{
		backend.New(backendServer.URL, "", 1),
	}
	bal := balancer.NewRoundRobin(backends)
	proxy := NewHTTPProxy(bal, 0, 10*time.Millisecond)

	// Verify initial connections
	if backends[0].GetActiveConnections() != 0 {
		t.Errorf("Expected 0 initial connections, got %d", backends[0].GetActiveConnections())
	}

	// Create request
	req := httptest.NewRequest("GET", "http://example.com/test", nil)
	w := httptest.NewRecorder()

	// Serve request
	proxy.ServeHTTP(w, req)

	// Connections should be decremented after request
	if backends[0].GetActiveConnections() != 0 {
		t.Errorf("Expected 0 connections after request, got %d", backends[0].GetActiveConnections())
	}
}

func TestGetClientIP_XForwardedFor(t *testing.T) {
	req := httptest.NewRequest("GET", "http://example.com/test", nil)
	req.Header.Set("X-Forwarded-For", "192.168.1.1, 10.0.0.1")

	ip := getClientIP(req)
	if ip != "192.168.1.1" {
		t.Errorf("Expected IP '192.168.1.1', got %s", ip)
	}
}

func TestGetClientIP_XRealIP(t *testing.T) {
	req := httptest.NewRequest("GET", "http://example.com/test", nil)
	req.Header.Set("X-Real-IP", "192.168.1.2")

	ip := getClientIP(req)
	if ip != "192.168.1.2" {
		t.Errorf("Expected IP '192.168.1.2', got %s", ip)
	}
}

func TestGetClientIP_XForwardedForPriority(t *testing.T) {
	req := httptest.NewRequest("GET", "http://example.com/test", nil)
	req.Header.Set("X-Forwarded-For", "192.168.1.1")
	req.Header.Set("X-Real-IP", "192.168.1.2")

	ip := getClientIP(req)
	if ip != "192.168.1.1" {
		t.Errorf("Expected X-Forwarded-For to take priority, got %s", ip)
	}
}

func TestGetClientIP_RemoteAddr(t *testing.T) {
	req := httptest.NewRequest("GET", "http://example.com/test", nil)
	req.RemoteAddr = "192.168.1.3:12345"

	ip := getClientIP(req)
	if ip != "192.168.1.3" {
		t.Errorf("Expected IP '192.168.1.3', got %s", ip)
	}
}

func TestCustomResponseWriter_WriteHeader(t *testing.T) {
	w := httptest.NewRecorder()
	crw := &customResponseWriter{ResponseWriter: w}

	crw.WriteHeader(http.StatusCreated)

	if crw.statusCode != http.StatusCreated {
		t.Errorf("Expected status code 201, got %d", crw.statusCode)
	}

	if w.Code != http.StatusCreated {
		t.Errorf("Expected underlying writer status 201, got %d", w.Code)
	}
}

func TestCustomResponseWriter_Write(t *testing.T) {
	w := httptest.NewRecorder()
	crw := &customResponseWriter{ResponseWriter: w}

	data := []byte("test data")
	n, err := crw.Write(data)

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if n != len(data) {
		t.Errorf("Expected to write %d bytes, wrote %d", len(data), n)
	}

	if crw.statusCode != http.StatusOK {
		t.Errorf("Expected default status code 200, got %d", crw.statusCode)
	}

	if w.Body.String() != "test data" {
		t.Errorf("Expected 'test data', got %s", w.Body.String())
	}
}

func TestCustomResponseWriter_WriteWithoutHeader(t *testing.T) {
	w := httptest.NewRecorder()
	crw := &customResponseWriter{ResponseWriter: w}

	// Write without calling WriteHeader first
	crw.Write([]byte("data"))

	// Should default to 200
	if crw.statusCode != http.StatusOK {
		t.Errorf("Expected default status 200, got %d", crw.statusCode)
	}
}

func TestHTTPProxy_MultipleBackends(t *testing.T) {
	// Create multiple backend servers
	backend1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("backend1"))
	}))
	defer backend1.Close()

	backend2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("backend2"))
	}))
	defer backend2.Close()

	// Create proxy
	backends := []*backend.Backend{
		backend.New(backend1.URL, "", 1),
		backend.New(backend2.URL, "", 1),
	}
	bal := balancer.NewRoundRobin(backends)
	proxy := NewHTTPProxy(bal, 0, 10*time.Millisecond)

	responses := make(map[string]int)

	// Make multiple requests
	for i := 0; i < 4; i++ {
		req := httptest.NewRequest("GET", "http://example.com/test", nil)
		w := httptest.NewRecorder()
		proxy.ServeHTTP(w, req)

		responses[w.Body.String()]++
	}

	// Should hit each backend twice
	if responses["backend1"] != 2 {
		t.Errorf("Expected 2 requests to backend1, got %d", responses["backend1"])
	}
	if responses["backend2"] != 2 {
		t.Errorf("Expected 2 requests to backend2, got %d", responses["backend2"])
	}
}

func TestHTTPProxy_InvalidBackendURL(t *testing.T) {
	// Create backend with invalid URL
	backends := []*backend.Backend{
		backend.New("://invalid-url", "", 1),
	}
	bal := balancer.NewRoundRobin(backends)
	proxy := NewHTTPProxy(bal, 2, 10*time.Millisecond)

	req := httptest.NewRequest("GET", "http://example.com/test", nil)
	w := httptest.NewRecorder()

	proxy.ServeHTTP(w, req)

	// Should return service unavailable
	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status 503, got %d", w.Code)
	}
}

func BenchmarkHTTPProxy_ServeHTTP(b *testing.B) {
	backendServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer backendServer.Close()

	backends := []*backend.Backend{
		backend.New(backendServer.URL, "", 1),
	}
	bal := balancer.NewRoundRobin(backends)
	proxy := NewHTTPProxy(bal, 0, 10*time.Millisecond)

	req := httptest.NewRequest("GET", "http://example.com/test", nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		proxy.ServeHTTP(w, req)
	}
}

func TestGetClientIP_EmptyHeaders(t *testing.T) {
	req := httptest.NewRequest("GET", "http://example.com/test", nil)
	req.RemoteAddr = "127.0.0.1:54321"

	ip := getClientIP(req)

	if ip != "127.0.0.1" {
		t.Errorf("Expected IP '127.0.0.1', got '%s'", ip)
	}
}

func TestGetClientIP_MultipleXForwardedFor(t *testing.T) {
	req := httptest.NewRequest("GET", "http://example.com/test", nil)
	req.Header.Set("X-Forwarded-For", "  192.168.1.100  ,  10.0.0.5  ")

	ip := getClientIP(req)

	if ip != "192.168.1.100" {
		t.Errorf("Expected IP '192.168.1.100' (trimmed), got '%s'", ip)
	}
}

func TestHTTPProxy_RequestHeaders(t *testing.T) {
	receivedHeaders := make(http.Header)

	backendServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for k, v := range r.Header {
			receivedHeaders[k] = v
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer backendServer.Close()

	backends := []*backend.Backend{
		backend.New(backendServer.URL, "", 1),
	}
	bal := balancer.NewRoundRobin(backends)
	proxy := NewHTTPProxy(bal, 0, 10*time.Millisecond)

	req := httptest.NewRequest("GET", "http://example.com/test", nil)
	req.Header.Set("User-Agent", "test-client")
	req.Header.Set("Custom-Header", "custom-value")

	w := httptest.NewRecorder()
	proxy.ServeHTTP(w, req)

	if receivedHeaders.Get("User-Agent") != "test-client" {
		t.Errorf("Expected User-Agent header to be forwarded")
	}

	if receivedHeaders.Get("Custom-Header") != "custom-value" {
		t.Errorf("Expected Custom-Header to be forwarded")
	}
}

func TestHTTPProxy_ResponseHeaders(t *testing.T) {
	backendServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Custom-Response", "backend-value")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	}))
	defer backendServer.Close()

	backends := []*backend.Backend{
		backend.New(backendServer.URL, "", 1),
	}
	bal := balancer.NewRoundRobin(backends)
	proxy := NewHTTPProxy(bal, 0, 10*time.Millisecond)

	req := httptest.NewRequest("GET", "http://example.com/test", nil)
	w := httptest.NewRecorder()

	proxy.ServeHTTP(w, req)

	if w.Header().Get("X-Custom-Response") != "backend-value" {
		t.Errorf("Expected response header X-Custom-Response to be preserved")
	}

	if w.Header().Get("Content-Type") != "application/json" {
		t.Errorf("Expected Content-Type header to be preserved")
	}
}

func TestHTTPProxy_DifferentHTTPMethods(t *testing.T) {
	methods := []string{"GET", "POST", "PUT", "DELETE", "PATCH", "HEAD", "OPTIONS"}

	for _, method := range methods {
		t.Run(fmt.Sprintf("Method_%s", method), func(t *testing.T) {
			receivedMethod := ""

			backendServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				receivedMethod = r.Method
				w.WriteHeader(http.StatusOK)
			}))
			defer backendServer.Close()

			backends := []*backend.Backend{
				backend.New(backendServer.URL, "", 1),
			}
			bal := balancer.NewRoundRobin(backends)
			proxy := NewHTTPProxy(bal, 0, 10*time.Millisecond)

			req := httptest.NewRequest(method, "http://example.com/test", nil)
			w := httptest.NewRecorder()

			proxy.ServeHTTP(w, req)

			if receivedMethod != method {
				t.Errorf("Expected method %s, backend received %s", method, receivedMethod)
			}
		})
	}
}

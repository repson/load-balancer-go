package proxy

import (
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"

	"github.com/isaac/load-balancer-go/internal/balancer"
	"github.com/isaac/load-balancer-go/internal/logger"
)

// HTTPProxy represents an HTTP reverse proxy with load balancing
type HTTPProxy struct {
	balancer   balancer.Balancer
	maxRetries int
	retryDelay time.Duration
}

// NewHTTPProxy creates a new HTTP proxy
func NewHTTPProxy(bal balancer.Balancer, maxRetries int, retryDelay time.Duration) *HTTPProxy {
	return &HTTPProxy{
		balancer:   bal,
		maxRetries: maxRetries,
		retryDelay: retryDelay,
	}
}

// ServeHTTP handles HTTP requests with load balancing and retry logic
func (p *HTTPProxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	clientIP := getClientIP(r)
	
	var lastErr error
	attempts := 0
	maxAttempts := p.maxRetries + 1

	for attempts < maxAttempts {
		// Get next backend
		backend, err := p.balancer.NextBackend(clientIP)
		if err != nil {
			logger.Error("Failed to get backend", "error", err)
			http.Error(w, "Service Unavailable", http.StatusServiceUnavailable)
			return
		}

		// Increment active connections
		backend.IncrementConnections()
		
		// Parse backend URL
		backendURL, err := url.Parse(backend.URL)
		if err != nil {
			backend.DecrementConnections()
			logger.Error("Invalid backend URL", "url", backend.URL, "error", err)
			lastErr = err
			attempts++
			if attempts < maxAttempts {
				time.Sleep(p.retryDelay)
			}
			continue
		}

		// Create reverse proxy
		proxy := httputil.NewSingleHostReverseProxy(backendURL)
		
		// Custom error handler to capture errors
		errorOccurred := false
		proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
			errorOccurred = true
			lastErr = err
			logger.Warn("Backend request failed",
				"backend", backend.URL,
				"attempt", attempts+1,
				"max_attempts", maxAttempts,
				"error", err)
		}

		// Track response for logging
		proxy.ModifyResponse = func(resp *http.Response) error {
			if resp.StatusCode >= 500 {
				errorOccurred = true
				lastErr = fmt.Errorf("backend returned %d", resp.StatusCode)
				return lastErr
			}
			return nil
		}

		// Create a custom response writer to check if we actually wrote response
		crw := &customResponseWriter{ResponseWriter: w, statusCode: 0}
		
		// Serve the request
		proxy.ServeHTTP(crw, r)
		
		// Decrement connections
		backend.DecrementConnections()

		// If no error occurred, we're done
		if !errorOccurred && crw.statusCode > 0 && crw.statusCode < 500 {
			logger.Info("Request proxied successfully",
				"backend", backend.URL,
				"client_ip", clientIP,
				"path", r.URL.Path,
				"status", crw.statusCode)
			return
		}

		// Retry logic
		attempts++
		if attempts < maxAttempts {
			logger.Info("Retrying request",
				"attempt", attempts,
				"max_attempts", maxAttempts,
				"delay", p.retryDelay)
			time.Sleep(p.retryDelay)
		}
	}

	// All retries failed
	logger.Error("All retry attempts failed",
		"attempts", attempts,
		"last_error", lastErr)
	http.Error(w, "Service Unavailable", http.StatusServiceUnavailable)
}

// customResponseWriter wraps http.ResponseWriter to capture status code
type customResponseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (w *customResponseWriter) WriteHeader(statusCode int) {
	w.statusCode = statusCode
	w.ResponseWriter.WriteHeader(statusCode)
}

func (w *customResponseWriter) Write(b []byte) (int, error) {
	if w.statusCode == 0 {
		w.statusCode = http.StatusOK
	}
	return w.ResponseWriter.Write(b)
}

// getClientIP extracts the client IP from the request
func getClientIP(r *http.Request) string {
	// Check X-Forwarded-For header
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		ips := strings.Split(xff, ",")
		if len(ips) > 0 {
			return strings.TrimSpace(ips[0])
		}
	}
	
	// Check X-Real-IP header
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}
	
	// Fall back to RemoteAddr
	ip := r.RemoteAddr
	if idx := strings.LastIndex(ip, ":"); idx != -1 {
		ip = ip[:idx]
	}
	return ip
}

package proxy

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
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

// ServeHTTP handles HTTP requests with load balancing and retry logic.
//
// Each attempt is captured into a buffer via httptest.ResponseRecorder so
// that a failed attempt never writes partial data to the real
// http.ResponseWriter. Only a fully successful response is flushed to the
// client. This prevents the double-write corruption that occurred when
// httputil.ReverseProxy wrote headers/body on one attempt and then a
// subsequent retry tried to write again to the same writer.
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

		// Buffer this attempt so we never write partial data to the real writer.
		rec := httptest.NewRecorder()

		// Create reverse proxy
		proxy := httputil.NewSingleHostReverseProxy(backendURL)

		errorOccurred := false
		proxy.ErrorHandler = func(_ http.ResponseWriter, _ *http.Request, err error) {
			errorOccurred = true
			lastErr = err
			logger.Warn("Backend request failed",
				"backend", backend.URL,
				"attempt", attempts+1,
				"max_attempts", maxAttempts,
				"error", err)
		}

		proxy.ModifyResponse = func(resp *http.Response) error {
			if resp.StatusCode >= 500 {
				errorOccurred = true
				lastErr = fmt.Errorf("backend returned %d", resp.StatusCode)
				return lastErr
			}
			return nil
		}

		proxy.ServeHTTP(rec, r)
		backend.DecrementConnections()

		// Only flush to the real writer when the attempt succeeded.
		if !errorOccurred && rec.Code < 500 {
			result := rec.Result()
			for key, values := range result.Header {
				for _, v := range values {
					w.Header().Add(key, v)
				}
			}
			w.WriteHeader(result.StatusCode)
			buf := new(bytes.Buffer)
			buf.ReadFrom(result.Body)
			result.Body.Close()
			w.Write(buf.Bytes())

			logger.Info("Request proxied successfully",
				"backend", backend.URL,
				"client_ip", clientIP,
				"path", r.URL.Path,
				"status", rec.Code)
			return
		}

		// Retry
		attempts++
		if attempts < maxAttempts {
			logger.Info("Retrying request",
				"attempt", attempts,
				"max_attempts", maxAttempts,
				"delay", p.retryDelay)
			time.Sleep(p.retryDelay)
		}
	}

	// All retries failed — nothing has been written to w yet.
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

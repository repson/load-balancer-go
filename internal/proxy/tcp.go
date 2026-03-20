package proxy

import (
	"fmt"
	"io"
	"net"
	"time"

	"github.com/isaac/load-balancer-go/internal/balancer"
	"github.com/isaac/load-balancer-go/internal/logger"
)

// TCPProxy represents a TCP proxy with load balancing
type TCPProxy struct {
	balancer    balancer.Balancer
	maxRetries  int
	retryDelay  time.Duration
	dialTimeout time.Duration
}

// NewTCPProxy creates a new TCP proxy.
// dialTimeout controls how long to wait when connecting to a backend;
// pass 0 to use the default of 5 seconds.
func NewTCPProxy(bal balancer.Balancer, maxRetries int, retryDelay time.Duration, dialTimeout time.Duration) *TCPProxy {
	if dialTimeout <= 0 {
		dialTimeout = 5 * time.Second
	}
	return &TCPProxy{
		balancer:    bal,
		maxRetries:  maxRetries,
		retryDelay:  retryDelay,
		dialTimeout: dialTimeout,
	}
}

// HandleConnection handles a TCP connection with load balancing and retry logic
func (p *TCPProxy) HandleConnection(clientConn net.Conn) {
	defer clientConn.Close()

	clientIP := getClientIPFromConn(clientConn)

	var lastErr error
	attempts := 0
	maxAttempts := p.maxRetries + 1

	for attempts < maxAttempts {
		// Get next backend
		backend, err := p.balancer.NextBackend(clientIP)
		if err != nil {
			logger.Error("Failed to get backend", "error", err)
			return
		}

		// Increment active connections
		backend.IncrementConnections()

		// Try to connect to backend
		backendConn, err := net.DialTimeout("tcp", backend.Address, p.dialTimeout)
		if err != nil {
			backend.DecrementConnections()
			lastErr = err
			logger.Warn("Backend connection failed",
				"backend", backend.Address,
				"attempt", attempts+1,
				"max_attempts", maxAttempts,
				"error", err)

			attempts++
			if attempts < maxAttempts {
				logger.Info("Retrying connection",
					"attempt", attempts,
					"max_attempts", maxAttempts,
					"delay", p.retryDelay)
				time.Sleep(p.retryDelay)
			}
			continue
		}

		// Connection successful
		logger.Info("TCP connection established",
			"backend", backend.Address,
			"client_ip", clientIP)

		// Proxy data bidirectionally
		errChan := make(chan error, 2)

		// Client -> Backend
		go func() {
			_, err := io.Copy(backendConn, clientConn)
			errChan <- err
		}()

		// Backend -> Client
		go func() {
			_, err := io.Copy(clientConn, backendConn)
			errChan <- err
		}()

		// Wait for the first direction to finish, then close both connections
		// so the second goroutine is unblocked, and finally drain its result.
		err = <-errChan
		backendConn.Close()
		clientConn.Close() // unblocks the second goroutine
		<-errChan          // wait for the second goroutine to exit
		backend.DecrementConnections()

		if err != nil && err != io.EOF {
			logger.Warn("TCP proxy error", "error", err, "backend", backend.Address)
		} else {
			logger.Info("TCP connection closed",
				"backend", backend.Address,
				"client_ip", clientIP)
		}

		// Connection was successful (even if it ended with error during transfer)
		return
	}

	// All retries failed
	logger.Error("All TCP connection attempts failed",
		"attempts", attempts,
		"last_error", lastErr,
		"client_ip", clientIP)
}

// getClientIPFromConn extracts the client IP from a TCP connection
func getClientIPFromConn(conn net.Conn) string {
	addr := conn.RemoteAddr().String()
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return addr
	}
	return host
}

// Serve starts the TCP proxy server
func (p *TCPProxy) Serve(listen string) error {
	listener, err := net.Listen("tcp", listen)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", listen, err)
	}
	defer listener.Close()

	logger.Info("TCP proxy listening", "address", listen)
	p.serveListener(listener)
	return nil
}

// serveListener runs the accept loop on an already-open listener.
// It returns only when the listener is closed.
func (p *TCPProxy) serveListener(listener net.Listener) {
	for {
		conn, err := listener.Accept()
		if err != nil {
			logger.Error("Failed to accept connection", "error", err)
			continue
		}

		// Handle each connection in a goroutine
		go p.HandleConnection(conn)
	}
}

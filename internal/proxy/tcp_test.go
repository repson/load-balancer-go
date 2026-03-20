package proxy

import (
	"bytes"
	"io"
	"net"
	"testing"
	"time"

	"github.com/isaac/load-balancer-go/internal/backend"
	"github.com/isaac/load-balancer-go/internal/balancer"
)

func TestNewTCPProxy(t *testing.T) {
	backends := []*backend.Backend{
		backend.New("", "localhost:4001", 1),
	}
	bal := balancer.NewRoundRobin(backends)

	proxy := NewTCPProxy(bal, 3, 100*time.Millisecond, 0)

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

func TestGetClientIPFromConn(t *testing.T) {
	// Create a mock connection
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to create listener: %v", err)
	}
	defer listener.Close()

	// Connect to listener
	done := make(chan bool)
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()

		ip := getClientIPFromConn(conn)
		if ip != "127.0.0.1" {
			t.Errorf("Expected IP '127.0.0.1', got '%s'", ip)
		}
		done <- true
	}()

	conn, err := net.Dial("tcp", listener.Addr().String())
	if err != nil {
		t.Fatalf("Failed to dial: %v", err)
	}
	conn.Close()

	<-done
}

func TestTCPProxy_HandleConnection_Success(t *testing.T) {
	// Create a test backend server
	backendListener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to create backend listener: %v", err)
	}
	defer backendListener.Close()

	backendAddr := backendListener.Addr().String()

	// Backend server echoes data
	go func() {
		conn, err := backendListener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()

		buf := make([]byte, 1024)
		n, _ := conn.Read(buf)
		conn.Write(buf[:n])
	}()

	// Create proxy
	backends := []*backend.Backend{
		backend.New("", backendAddr, 1),
	}
	bal := balancer.NewRoundRobin(backends)
	proxy := NewTCPProxy(bal, 0, 10*time.Millisecond, 0)

	// Create client connection
	clientConn, serverConn := net.Pipe()

	// Handle connection in goroutine
	go proxy.HandleConnection(serverConn)

	// Send data from client
	testData := []byte("hello backend")
	clientConn.Write(testData)

	// Read response
	buf := make([]byte, 1024)
	clientConn.SetReadDeadline(time.Now().Add(1 * time.Second))
	n, _ := clientConn.Read(buf)

	if !bytes.Equal(buf[:n], testData) {
		t.Errorf("Expected echo of '%s', got '%s'", testData, buf[:n])
	}

	clientConn.Close()
}

func TestTCPProxy_HandleConnection_NoBackends(t *testing.T) {
	// Create proxy with no backends
	backends := []*backend.Backend{}
	bal := balancer.NewRoundRobin(backends)
	proxy := NewTCPProxy(bal, 0, 10*time.Millisecond, 0)

	// Create client connection
	clientConn, serverConn := net.Pipe()

	// Handle connection
	go proxy.HandleConnection(serverConn)

	// Try to write
	clientConn.Write([]byte("test"))

	// Connection should be closed
	buf := make([]byte, 1024)
	clientConn.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
	_, err := clientConn.Read(buf)

	if err != io.EOF {
		// Connection should be closed
		clientConn.Close()
	}
}

func TestTCPProxy_HandleConnection_BackendUnavailable(t *testing.T) {
	// Create proxy with unavailable backend
	backends := []*backend.Backend{
		backend.New("", "127.0.0.1:99999", 1), // Invalid port
	}
	bal := balancer.NewRoundRobin(backends)
	proxy := NewTCPProxy(bal, 1, 10*time.Millisecond, 0)

	// Create client connection
	clientConn, serverConn := net.Pipe()

	// Handle connection
	done := make(chan bool)
	go func() {
		proxy.HandleConnection(serverConn)
		done <- true
	}()

	// Connection should fail and close
	select {
	case <-done:
		// Good, connection was handled and closed
	case <-time.After(500 * time.Millisecond):
		t.Error("HandleConnection did not complete in time")
	}

	clientConn.Close()
}

func TestTCPProxy_HandleConnection_Retry(t *testing.T) {
	// This test verifies retry behavior
	backends := []*backend.Backend{
		backend.New("", "127.0.0.1:99998", 1), // Unavailable backend
	}
	bal := balancer.NewRoundRobin(backends)
	proxy := NewTCPProxy(bal, 2, 10*time.Millisecond, 0)

	clientConn, serverConn := net.Pipe()

	start := time.Now()

	done := make(chan bool)
	go func() {
		proxy.HandleConnection(serverConn)
		done <- true
	}()

	select {
	case <-done:
		elapsed := time.Since(start)
		// Should have retried at least once (2 retries = 2 delays minimum)
		// But we can't be too strict on timing
		if elapsed < 10*time.Millisecond {
			t.Error("Expected retry delays, but completed too quickly")
		}
	case <-time.After(1 * time.Second):
		t.Error("Retry logic took too long")
	}

	clientConn.Close()
}

func TestTCPProxy_ConnectionCounting(t *testing.T) {
	// Create a test backend server
	backendListener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to create backend listener: %v", err)
	}
	defer backendListener.Close()

	backendAddr := backendListener.Addr().String()

	// Backend accepts and closes immediately
	go func() {
		conn, _ := backendListener.Accept()
		if conn != nil {
			time.Sleep(50 * time.Millisecond)
			conn.Close()
		}
	}()

	backends := []*backend.Backend{
		backend.New("", backendAddr, 1),
	}

	if backends[0].GetActiveConnections() != 0 {
		t.Errorf("Expected 0 initial connections, got %d", backends[0].GetActiveConnections())
	}

	bal := balancer.NewRoundRobin(backends)
	proxy := NewTCPProxy(bal, 0, 10*time.Millisecond, 0)

	clientConn, serverConn := net.Pipe()

	// Use channel to signal when connection is established
	established := make(chan bool, 1)
	done := make(chan bool)

	go func() {
		// Signal when starting to handle connection
		established <- true
		proxy.HandleConnection(serverConn)
		done <- true
	}()

	// Wait for connection to be established
	<-established

	// Give a brief moment for the connection to fully establish
	time.Sleep(10 * time.Millisecond)

	// Close client connection
	clientConn.Close()

	// Wait for handler to finish
	<-done

	// Verify connections are back to 0 (with small retry loop instead of sleep)
	for i := 0; i < 10; i++ {
		if backends[0].GetActiveConnections() == 0 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}

	if backends[0].GetActiveConnections() != 0 {
		t.Errorf("Expected 0 connections after close, got %d", backends[0].GetActiveConnections())
	}
}

func TestTCPProxy_BidirectionalData(t *testing.T) {
	// Create a backend that echoes everything
	backendListener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to create backend listener: %v", err)
	}
	defer backendListener.Close()

	backendAddr := backendListener.Addr().String()

	// Backend echoes data
	go func() {
		conn, err := backendListener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		io.Copy(conn, conn)
	}()

	backends := []*backend.Backend{
		backend.New("", backendAddr, 1),
	}
	bal := balancer.NewRoundRobin(backends)
	proxy := NewTCPProxy(bal, 0, 10*time.Millisecond, 0)

	clientConn, serverConn := net.Pipe()

	go proxy.HandleConnection(serverConn)

	// Send multiple messages
	messages := []string{"hello", "world", "test"}
	for _, msg := range messages {
		clientConn.Write([]byte(msg))

		buf := make([]byte, 1024)
		clientConn.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
		n, err := clientConn.Read(buf)
		if err != nil {
			t.Fatalf("Failed to read response: %v", err)
		}

		if string(buf[:n]) != msg {
			t.Errorf("Expected '%s', got '%s'", msg, string(buf[:n]))
		}
	}

	clientConn.Close()
}

func TestTCPProxy_Serve(t *testing.T) {
	backends := []*backend.Backend{
		backend.New("", "localhost:9999", 1),
	}
	bal := balancer.NewRoundRobin(backends)
	proxy := NewTCPProxy(bal, 0, 10*time.Millisecond, 0)

	// Start server in goroutine
	errChan := make(chan error, 1)
	go func() {
		err := proxy.Serve("127.0.0.1:0")
		errChan <- err
	}()

	// Give it a moment to start
	time.Sleep(50 * time.Millisecond)

	// We can't easily test the full Serve functionality without a way to stop it
	// So we'll just verify it doesn't error immediately
	select {
	case err := <-errChan:
		if err != nil {
			t.Errorf("Serve returned error: %v", err)
		}
	case <-time.After(100 * time.Millisecond):
		// Good, server is running
	}
}

func TestTCPProxy_Serve_InvalidAddress(t *testing.T) {
	backends := []*backend.Backend{
		backend.New("", "localhost:9999", 1),
	}
	bal := balancer.NewRoundRobin(backends)
	proxy := NewTCPProxy(bal, 0, 10*time.Millisecond, 0)

	err := proxy.Serve("invalid:address:format")
	if err == nil {
		t.Error("Expected error for invalid address, got nil")
	}
}

func TestGetClientIPFromConn_NoPort(t *testing.T) {
	// Create a mock connection with unusual address format
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to create listener: %v", err)
	}
	defer listener.Close()

	done := make(chan string)
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			done <- ""
			return
		}
		defer conn.Close()

		ip := getClientIPFromConn(conn)
		done <- ip
	}()

	conn, err := net.Dial("tcp", listener.Addr().String())
	if err != nil {
		t.Fatalf("Failed to dial: %v", err)
	}
	defer conn.Close()

	ip := <-done

	// Should extract IP without port
	if ip == "" {
		t.Error("Expected non-empty IP")
	}
}

func BenchmarkTCPProxy_HandleConnection(b *testing.B) {
	backendListener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		b.Fatalf("Failed to create backend listener: %v", err)
	}
	defer backendListener.Close()

	backendAddr := backendListener.Addr().String()

	go func() {
		for {
			conn, err := backendListener.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				io.Copy(c, c)
			}(conn)
		}
	}()

	backends := []*backend.Backend{
		backend.New("", backendAddr, 1),
	}
	bal := balancer.NewRoundRobin(backends)
	proxy := NewTCPProxy(bal, 0, 10*time.Millisecond, 0)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		clientConn, serverConn := net.Pipe()

		go proxy.HandleConnection(serverConn)

		clientConn.Write([]byte("test"))
		buf := make([]byte, 4)
		clientConn.Read(buf)
		clientConn.Close()
	}
}

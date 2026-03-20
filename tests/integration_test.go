package tests

import (
	"bytes"
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/isaac/load-balancer-go/internal/backend"
	"github.com/isaac/load-balancer-go/internal/balancer"
	"github.com/isaac/load-balancer-go/internal/proxy"
)

// TestEndToEnd_HTTP_And_TCP tests the complete system with both HTTP and TCP proxies running
func TestEndToEnd_HTTP_And_TCP(t *testing.T) {
	// Setup: Create backend servers

	// HTTP backend servers
	httpBackend1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("backend1"))
	}))
	defer httpBackend1.Close()

	httpBackend2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("backend2"))
	}))
	defer httpBackend2.Close()

	// TCP backend servers
	tcpBackend1 := createTCPBackend(t)
	defer tcpBackend1.Close()

	tcpBackend2 := createTCPBackend(t)
	defer tcpBackend2.Close()

	// Create HTTP proxy
	httpBackends := []*backend.Backend{
		backend.New(httpBackend1.URL, "", 1),
		backend.New(httpBackend2.URL, "", 1),
	}
	httpBalancer := balancer.NewRoundRobin(httpBackends)
	httpProxy := proxy.NewHTTPProxy(httpBalancer, 3, 100*time.Millisecond)

	// Start HTTP proxy server on an OS-assigned port.
	httpProxyListener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to start HTTP proxy listener: %v", err)
	}
	httpProxyAddr := httpProxyListener.Addr().String()

	httpProxyServer := &http.Server{Handler: httpProxy}
	go func() {
		if err := httpProxyServer.Serve(httpProxyListener); err != nil && err != http.ErrServerClosed {
			t.Logf("HTTP proxy server error: %v", err)
		}
	}()
	defer httpProxyServer.Shutdown(context.Background())

	// Create TCP proxy
	tcpBackends := []*backend.Backend{
		backend.New("", tcpBackend1.Addr().String(), 1),
		backend.New("", tcpBackend2.Addr().String(), 1),
	}
	tcpBalancer := balancer.NewRoundRobin(tcpBackends)
	tcpProxy := proxy.NewTCPProxy(tcpBalancer, 3, 100*time.Millisecond, 0)

	// Start TCP proxy server on an OS-assigned port.
	tcpProxyListener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to start TCP proxy listener: %v", err)
	}
	tcpProxyAddr := tcpProxyListener.Addr().String()
	defer tcpProxyListener.Close()

	go func() {
		for {
			conn, err := tcpProxyListener.Accept()
			if err != nil {
				return
			}
			go tcpProxy.HandleConnection(conn)
		}
	}()

	// Test HTTP proxy
	t.Run("HTTP_LoadBalancing", func(t *testing.T) {
		responses := make(map[string]int)

		// Make multiple requests
		for i := 0; i < 4; i++ {
			resp, err := http.Get("http://" + httpProxyAddr + "/test")
			if err != nil {
				t.Fatalf("HTTP request failed: %v", err)
			}

			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()

			responses[string(body)]++
		}

		// Verify round-robin distribution
		if responses["backend1"] != 2 {
			t.Errorf("Expected 2 requests to backend1, got %d", responses["backend1"])
		}
		if responses["backend2"] != 2 {
			t.Errorf("Expected 2 requests to backend2, got %d", responses["backend2"])
		}
	})

	// Test TCP proxy
	t.Run("TCP_LoadBalancing", func(t *testing.T) {
		// Make multiple TCP connections
		for i := 0; i < 4; i++ {
			conn, err := net.Dial("tcp", tcpProxyAddr)
			if err != nil {
				t.Fatalf("TCP connection failed: %v", err)
			}

			// Send data
			testData := []byte("hello")
			_, err = conn.Write(testData)
			if err != nil {
				t.Fatalf("TCP write failed: %v", err)
			}

			// Read response
			buf := make([]byte, 1024)
			conn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
			n, err := conn.Read(buf)
			if err != nil {
				t.Fatalf("TCP read failed: %v", err)
			}

			// Verify echo
			if !bytes.Equal(buf[:n], testData) {
				t.Errorf("Expected echo of '%s', got '%s'", testData, buf[:n])
			}

			conn.Close()
		}
	})

	// Test concurrent requests
	t.Run("Concurrent_HTTP_Requests", func(t *testing.T) {
		done := make(chan bool, 10)

		for i := 0; i < 10; i++ {
			go func(id int) {
				resp, err := http.Get("http://" + httpProxyAddr + "/concurrent")
				if err != nil {
					t.Errorf("Concurrent request %d failed: %v", id, err)
					done <- false
					return
				}
				resp.Body.Close()

				if resp.StatusCode != http.StatusOK {
					t.Errorf("Expected status 200, got %d", resp.StatusCode)
					done <- false
					return
				}
				done <- true
			}(i)
		}

		// Wait for all requests
		for i := 0; i < 10; i++ {
			<-done
		}
	})

	// Test concurrent TCP connections
	t.Run("Concurrent_TCP_Connections", func(t *testing.T) {
		done := make(chan bool, 10)

		for i := 0; i < 10; i++ {
			go func(id int) {
				conn, err := net.Dial("tcp", tcpProxyAddr)
				if err != nil {
					t.Errorf("Concurrent TCP connection %d failed: %v", id, err)
					done <- false
					return
				}
				defer conn.Close()

				testData := []byte("concurrent")
				conn.Write(testData)

				buf := make([]byte, 1024)
				conn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
				n, _ := conn.Read(buf)

				if bytes.Equal(buf[:n], testData) {
					done <- true
				} else {
					done <- false
				}
			}(i)
		}

		// Wait for all connections
		for i := 0; i < 10; i++ {
			<-done
		}
	})
}

// Helper: Create TCP backend server (echo server)
func createTCPBackend(t *testing.T) net.Listener {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to create TCP backend listener: %v", err)
	}

	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				io.Copy(c, c) // Echo server
			}(conn)
		}
	}()

	return listener
}

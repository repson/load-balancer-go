package balancer

import (
	"errors"
	"testing"

	"github.com/isaac/load-balancer-go/internal/backend"
)

func TestMockBalancer_DefaultBehavior(t *testing.T) {
	mock := NewMockBalancer()

	_, err := mock.NextBackend("192.168.1.1")
	if err == nil {
		t.Error("Expected error from default mock behavior")
	}

	if mock.GetCallCount() != 1 {
		t.Errorf("Expected call count 1, got %d", mock.GetCallCount())
	}

	if mock.GetLastClientIP() != "192.168.1.1" {
		t.Errorf("Expected last client IP '192.168.1.1', got '%s'", mock.GetLastClientIP())
	}
}

func TestMockBalancer_SetBackend(t *testing.T) {
	mock := NewMockBalancer()
	testBackend := backend.New("http://test-backend", "", 1)

	mock.SetBackend(testBackend)

	b, err := mock.NextBackend("192.168.1.1")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if b != testBackend {
		t.Error("Expected to receive the configured test backend")
	}
}

func TestMockBalancer_SetError(t *testing.T) {
	mock := NewMockBalancer()
	expectedErr := errors.New("custom test error")

	mock.SetError(expectedErr)

	_, err := mock.NextBackend("192.168.1.1")
	if err == nil {
		t.Error("Expected error")
	}

	if err.Error() != expectedErr.Error() {
		t.Errorf("Expected error '%v', got '%v'", expectedErr, err)
	}
}

func TestMockBalancer_SetNextBackendFunc(t *testing.T) {
	mock := NewMockBalancer()
	backend1 := backend.New("http://backend1", "", 1)
	backend2 := backend.New("http://backend2", "", 1)

	// Custom function that returns different backends based on IP
	mock.SetNextBackendFunc(func(clientIP string) (*backend.Backend, error) {
		if clientIP == "192.168.1.1" {
			return backend1, nil
		}
		return backend2, nil
	})

	b1, err := mock.NextBackend("192.168.1.1")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if b1 != backend1 {
		t.Error("Expected backend1 for IP 192.168.1.1")
	}

	b2, err := mock.NextBackend("192.168.1.2")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if b2 != backend2 {
		t.Error("Expected backend2 for IP 192.168.1.2")
	}

	if mock.GetCallCount() != 2 {
		t.Errorf("Expected call count 2, got %d", mock.GetCallCount())
	}
}

func TestMockBalancer_Reset(t *testing.T) {
	mock := NewMockBalancer()
	testBackend := backend.New("http://test", "", 1)

	mock.SetBackend(testBackend)
	mock.NextBackend("192.168.1.1")
	mock.NextBackend("192.168.1.2")

	if mock.GetCallCount() != 2 {
		t.Errorf("Expected call count 2 before reset, got %d", mock.GetCallCount())
	}

	mock.Reset()

	if mock.GetCallCount() != 0 {
		t.Errorf("Expected call count 0 after reset, got %d", mock.GetCallCount())
	}

	if mock.GetLastClientIP() != "" {
		t.Errorf("Expected empty last client IP after reset, got '%s'", mock.GetLastClientIP())
	}

	// After reset, should return to default behavior (error)
	_, err := mock.NextBackend("192.168.1.3")
	if err == nil {
		t.Error("Expected error after reset")
	}
}

func TestMockBalancer_ThreadSafety(t *testing.T) {
	mock := NewMockBalancer()
	testBackend := backend.New("http://test", "", 1)
	mock.SetBackend(testBackend)

	done := make(chan bool)
	calls := 100

	for i := 0; i < calls; i++ {
		go func(id int) {
			mock.NextBackend("192.168.1.1")
			done <- true
		}(i)
	}

	for i := 0; i < calls; i++ {
		<-done
	}

	if mock.GetCallCount() != calls {
		t.Errorf("Expected call count %d, got %d", calls, mock.GetCallCount())
	}
}

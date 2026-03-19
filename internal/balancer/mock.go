package balancer

import (
	"errors"
	"sync"

	"github.com/isaac/load-balancer-go/internal/backend"
)

// MockBalancer is a mock implementation of the Balancer interface for testing
type MockBalancer struct {
	mu              sync.Mutex
	nextBackendFunc func(clientIP string) (*backend.Backend, error)
	callCount       int
	lastClientIP    string
}

// NewMockBalancer creates a new mock balancer
func NewMockBalancer() *MockBalancer {
	return &MockBalancer{
		callCount: 0,
	}
}

// NextBackend implements the Balancer interface
func (m *MockBalancer) NextBackend(clientIP string) (*backend.Backend, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.callCount++
	m.lastClientIP = clientIP

	if m.nextBackendFunc != nil {
		return m.nextBackendFunc(clientIP)
	}

	// Default behavior: return error
	return nil, errors.New("mock balancer: no backend configured")
}

// SetNextBackendFunc sets a custom function for NextBackend
func (m *MockBalancer) SetNextBackendFunc(f func(clientIP string) (*backend.Backend, error)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.nextBackendFunc = f
}

// SetBackend configures the mock to always return the same backend
func (m *MockBalancer) SetBackend(b *backend.Backend) {
	m.SetNextBackendFunc(func(clientIP string) (*backend.Backend, error) {
		return b, nil
	})
}

// SetError configures the mock to always return an error
func (m *MockBalancer) SetError(err error) {
	m.SetNextBackendFunc(func(clientIP string) (*backend.Backend, error) {
		return nil, err
	})
}

// GetCallCount returns the number of times NextBackend was called
func (m *MockBalancer) GetCallCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.callCount
}

// GetLastClientIP returns the last client IP passed to NextBackend
func (m *MockBalancer) GetLastClientIP() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.lastClientIP
}

// Reset resets the mock state
func (m *MockBalancer) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.callCount = 0
	m.lastClientIP = ""
	m.nextBackendFunc = nil
}

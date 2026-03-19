package backend

import (
	"sync"
	"sync/atomic"
)

// Backend represents a backend server
type Backend struct {
	URL         string // For HTTP backends
	Address     string // For TCP backends
	Weight      int    // Weight for weighted algorithms
	activeConns int64  // Atomic counter for active connections
	mu          sync.RWMutex
}

// New creates a new Backend instance
func New(url, address string, weight int) *Backend {
	if weight <= 0 {
		weight = 1
	}
	return &Backend{
		URL:     url,
		Address: address,
		Weight:  weight,
	}
}

// IncrementConnections increments the active connections counter
func (b *Backend) IncrementConnections() {
	atomic.AddInt64(&b.activeConns, 1)
}

// DecrementConnections decrements the active connections counter
func (b *Backend) DecrementConnections() {
	atomic.AddInt64(&b.activeConns, -1)
}

// GetActiveConnections returns the current number of active connections
func (b *Backend) GetActiveConnections() int64 {
	return atomic.LoadInt64(&b.activeConns)
}

// String returns a string representation of the backend
func (b *Backend) String() string {
	if b.URL != "" {
		return b.URL
	}
	return b.Address
}

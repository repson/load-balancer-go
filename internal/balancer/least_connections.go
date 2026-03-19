package balancer

import (
	"sync"

	"github.com/isaac/load-balancer-go/internal/backend"
)

// LeastConnections implements the least connections load balancing algorithm
type LeastConnections struct {
	backends []*backend.Backend
	mu       sync.RWMutex
}

// NewLeastConnections creates a new LeastConnections balancer
func NewLeastConnections(backends []*backend.Backend) *LeastConnections {
	return &LeastConnections{
		backends: backends,
	}
}

// NextBackend returns the backend with the least active connections
func (lc *LeastConnections) NextBackend(clientIP string) (*backend.Backend, error) {
	lc.mu.RLock()
	defer lc.mu.RUnlock()

	if len(lc.backends) == 0 {
		return nil, ErrNoBackends
	}

	// Find backend with minimum connections
	var selected *backend.Backend
	minConns := int64(-1)

	for _, b := range lc.backends {
		conns := b.GetActiveConnections()
		if minConns == -1 || conns < minConns {
			minConns = conns
			selected = b
		}
	}

	return selected, nil
}

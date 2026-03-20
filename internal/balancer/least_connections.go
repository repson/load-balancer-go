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

// NextBackend returns the backend with the least active connections.
//
// Note on TOCTOU: the selection is a snapshot — between reading the
// connection count and the caller incrementing it (via
// Backend.IncrementConnections), another goroutine may have already
// incremented the same backend's counter. Under high concurrency the
// "fewest connections" guarantee therefore becomes a best-effort
// approximation rather than a strict invariant. This is an inherent
// trade-off of the algorithm and does not cause correctness issues.
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

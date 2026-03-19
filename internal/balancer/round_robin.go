package balancer

import (
	"sync/atomic"

	"github.com/isaac/load-balancer-go/internal/backend"
)

// RoundRobin implements the round-robin load balancing algorithm
type RoundRobin struct {
	backends []*backend.Backend
	counter  atomic.Uint64
}

// NewRoundRobin creates a new RoundRobin balancer
func NewRoundRobin(backends []*backend.Backend) *RoundRobin {
	return &RoundRobin{
		backends: backends,
	}
}

// NextBackend returns the next backend using round-robin algorithm
func (rr *RoundRobin) NextBackend(clientIP string) (*backend.Backend, error) {
	if len(rr.backends) == 0 {
		return nil, ErrNoBackends
	}

	// Get next index using atomic increment
	idx := rr.counter.Add(1) - 1
	return rr.backends[idx%uint64(len(rr.backends))], nil
}

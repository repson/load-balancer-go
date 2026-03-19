package balancer

import (
	"sync/atomic"

	"github.com/isaac/load-balancer-go/internal/backend"
)

// WeightedRoundRobin implements the weighted round-robin load balancing algorithm
type WeightedRoundRobin struct {
	backends []*backend.Backend
	expanded []*backend.Backend // Expanded list based on weights
	counter  atomic.Uint64
}

// NewWeightedRoundRobin creates a new WeightedRoundRobin balancer
func NewWeightedRoundRobin(backends []*backend.Backend) *WeightedRoundRobin {
	// Expand backends based on their weights
	expanded := make([]*backend.Backend, 0)
	for _, b := range backends {
		weight := b.Weight
		if weight <= 0 {
			weight = 1
		}
		// Add backend 'weight' times to the expanded list
		for i := 0; i < weight; i++ {
			expanded = append(expanded, b)
		}
	}

	return &WeightedRoundRobin{
		backends: backends,
		expanded: expanded,
	}
}

// NextBackend returns the next backend using weighted round-robin algorithm
func (wrr *WeightedRoundRobin) NextBackend(clientIP string) (*backend.Backend, error) {
	if len(wrr.expanded) == 0 {
		return nil, ErrNoBackends
	}

	// Get next index using atomic increment
	idx := wrr.counter.Add(1) - 1
	return wrr.expanded[idx%uint64(len(wrr.expanded))], nil
}

package balancer

import (
	"math/rand"
	"sync"

	"github.com/isaac/load-balancer-go/internal/backend"
)

// Random implements the random load balancing algorithm.
// It is safe for concurrent use.
type Random struct {
	backends []*backend.Backend
	mu       sync.Mutex
	rng      *rand.Rand
}

// NewRandom creates a new Random balancer
func NewRandom(backends []*backend.Backend) *Random {
	// rand.New with a fixed source is not thread-safe on its own; the mutex
	// in the struct protects concurrent access to rng.
	return &Random{
		backends: backends,
		rng:      rand.New(rand.NewSource(rand.Int63())),
	}
}

// NextBackend returns a random backend
func (r *Random) NextBackend(clientIP string) (*backend.Backend, error) {
	if len(r.backends) == 0 {
		return nil, ErrNoBackends
	}

	r.mu.Lock()
	idx := r.rng.Intn(len(r.backends))
	r.mu.Unlock()

	return r.backends[idx], nil
}

package balancer

import (
	"math/rand"
	"time"

	"github.com/isaac/load-balancer-go/internal/backend"
)

// Random implements the random load balancing algorithm
type Random struct {
	backends []*backend.Backend
	rng      *rand.Rand
}

// NewRandom creates a new Random balancer
func NewRandom(backends []*backend.Backend) *Random {
	return &Random{
		backends: backends,
		rng:      rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// NextBackend returns a random backend
func (r *Random) NextBackend(clientIP string) (*backend.Backend, error) {
	if len(r.backends) == 0 {
		return nil, ErrNoBackends
	}

	// Select a random backend
	idx := r.rng.Intn(len(r.backends))
	return r.backends[idx], nil
}

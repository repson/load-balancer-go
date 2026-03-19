package balancer

import (
	"hash/fnv"

	"github.com/isaac/load-balancer-go/internal/backend"
)

// IPHash implements the IP hash load balancing algorithm
type IPHash struct {
	backends []*backend.Backend
}

// NewIPHash creates a new IPHash balancer
func NewIPHash(backends []*backend.Backend) *IPHash {
	return &IPHash{
		backends: backends,
	}
}

// NextBackend returns a backend based on the hash of the client IP
func (ih *IPHash) NextBackend(clientIP string) (*backend.Backend, error) {
	if len(ih.backends) == 0 {
		return nil, ErrNoBackends
	}

	// Hash the client IP
	h := fnv.New64a()
	h.Write([]byte(clientIP))
	hash := h.Sum64()

	// Select backend based on hash modulo
	idx := hash % uint64(len(ih.backends))
	return ih.backends[idx], nil
}

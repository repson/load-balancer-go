package balancer

import (
	"fmt"

	"github.com/isaac/load-balancer-go/internal/backend"
)

// Balancer is the interface that all load balancing algorithms must implement
type Balancer interface {
	// NextBackend returns the next backend to use based on the algorithm
	// clientIP is used for algorithms that need client information (e.g., IP hash)
	NextBackend(clientIP string) (*backend.Backend, error)
}

var (
	// ErrNoBackends is returned when no backends are available
	ErrNoBackends = fmt.Errorf("no backends available")
)

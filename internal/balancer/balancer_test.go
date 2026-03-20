package balancer

import (
	"fmt"
	"sync"
	"testing"

	"github.com/isaac/load-balancer-go/internal/backend"
)

func TestRoundRobin_SequentialDistribution(t *testing.T) {
	backends := []*backend.Backend{
		backend.New("http://backend1", "", 1),
		backend.New("http://backend2", "", 1),
		backend.New("http://backend3", "", 1),
	}

	rr := NewRoundRobin(backends)

	// Test sequential distribution
	expected := []string{
		"http://backend1",
		"http://backend2",
		"http://backend3",
		"http://backend1",
		"http://backend2",
		"http://backend3",
	}

	for i, exp := range expected {
		b, err := rr.NextBackend("")
		if err != nil {
			t.Fatalf("Unexpected error at iteration %d: %v", i, err)
		}
		if b.URL != exp {
			t.Errorf("Iteration %d: expected %s, got %s", i, exp, b.URL)
		}
	}
}

func TestRoundRobin_NoBackends(t *testing.T) {
	rr := NewRoundRobin([]*backend.Backend{})
	_, err := rr.NextBackend("")
	if err != ErrNoBackends {
		t.Errorf("Expected ErrNoBackends, got %v", err)
	}
}

func TestRoundRobin_Concurrent(t *testing.T) {
	backends := []*backend.Backend{
		backend.New("http://backend1", "", 1),
		backend.New("http://backend2", "", 1),
		backend.New("http://backend3", "", 1),
	}

	rr := NewRoundRobin(backends)

	// Count distribution
	counts := make(map[string]int)
	var mu sync.Mutex

	var wg sync.WaitGroup
	requests := 300

	for i := 0; i < requests; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			b, err := rr.NextBackend("")
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}
			mu.Lock()
			counts[b.URL]++
			mu.Unlock()
		}()
	}

	wg.Wait()

	// Each backend should get exactly 100 requests
	for _, b := range backends {
		if counts[b.URL] != 100 {
			t.Errorf("Backend %s: expected 100 requests, got %d", b.URL, counts[b.URL])
		}
	}
}

func TestLeastConnections_SelectsMinimum(t *testing.T) {
	backends := []*backend.Backend{
		backend.New("http://backend1", "", 1),
		backend.New("http://backend2", "", 1),
		backend.New("http://backend3", "", 1),
	}

	// Simulate different connection counts
	backends[0].IncrementConnections()
	backends[0].IncrementConnections() // 2 connections
	backends[1].IncrementConnections() // 1 connection
	// backends[2] has 0 connections

	lc := NewLeastConnections(backends)

	b, err := lc.NextBackend("")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if b.URL != "http://backend3" {
		t.Errorf("Expected backend3 (0 connections), got %s", b.URL)
	}
}

func TestLeastConnections_ThreadSafety(t *testing.T) {
	backends := []*backend.Backend{
		backend.New("http://backend1", "", 1),
		backend.New("http://backend2", "", 1),
		backend.New("http://backend3", "", 1),
	}

	lc := NewLeastConnections(backends)

	var wg sync.WaitGroup
	requests := 1000

	for i := 0; i < requests; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			b, err := lc.NextBackend("")
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}
			// Simulate connection lifecycle
			b.IncrementConnections()
			b.DecrementConnections()
		}()
	}

	wg.Wait()

	// All connections should be closed
	for _, b := range backends {
		if b.GetActiveConnections() != 0 {
			t.Errorf("Backend %s: expected 0 connections, got %d", b.URL, b.GetActiveConnections())
		}
	}
}

func TestWeightedRoundRobin_Distribution(t *testing.T) {
	backends := []*backend.Backend{
		backend.New("http://backend1", "", 2), // weight 2
		backend.New("http://backend2", "", 1), // weight 1
		backend.New("http://backend3", "", 1), // weight 1
	}

	wrr := NewWeightedRoundRobin(backends)

	counts := make(map[string]int)
	requests := 400

	for i := 0; i < requests; i++ {
		b, err := wrr.NextBackend("")
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		counts[b.URL]++
	}

	// Backend1 should get 50% (weight 2/4)
	// Backend2 should get 25% (weight 1/4)
	// Backend3 should get 25% (weight 1/4)
	if counts["http://backend1"] != 200 {
		t.Errorf("Backend1: expected 200, got %d", counts["http://backend1"])
	}
	if counts["http://backend2"] != 100 {
		t.Errorf("Backend2: expected 100, got %d", counts["http://backend2"])
	}
	if counts["http://backend3"] != 100 {
		t.Errorf("Backend3: expected 100, got %d", counts["http://backend3"])
	}
}

func TestIPHash_Consistency(t *testing.T) {
	backends := []*backend.Backend{
		backend.New("http://backend1", "", 1),
		backend.New("http://backend2", "", 1),
		backend.New("http://backend3", "", 1),
	}

	ih := NewIPHash(backends)

	// Same IP should always get same backend
	clientIPs := []string{"192.168.1.1", "192.168.1.2", "192.168.1.3"}

	for _, ip := range clientIPs {
		var firstBackend string
		for i := 0; i < 10; i++ {
			b, err := ih.NextBackend(ip)
			if err != nil {
				t.Fatalf("Unexpected error for IP %s: %v", ip, err)
			}
			if i == 0 {
				firstBackend = b.URL
			} else if b.URL != firstBackend {
				t.Errorf("IP %s: inconsistent backend selection. First: %s, Got: %s", ip, firstBackend, b.URL)
			}
		}
	}
}

func TestIPHash_Distribution(t *testing.T) {
	backends := []*backend.Backend{
		backend.New("http://backend1", "", 1),
		backend.New("http://backend2", "", 1),
		backend.New("http://backend3", "", 1),
	}

	ih := NewIPHash(backends)

	counts := make(map[string]int)

	// Generate 300 different IPs
	for i := 0; i < 300; i++ {
		ip := fmt.Sprintf("192.168.%d.%d", i/256, i%256)
		b, err := ih.NextBackend(ip)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		counts[b.URL]++
	}

	// Check that all backends received some requests (distribution exists)
	for _, backend := range backends {
		if counts[backend.URL] == 0 {
			t.Errorf("Backend %s received no requests", backend.URL)
		}
	}

	// With 300 IPs and 3 backends, expect roughly 100 each (allow some variance)
	for url, count := range counts {
		if count < 50 || count > 150 {
			t.Errorf("Backend %s: distribution out of range: %d (expected ~100)", url, count)
		}
	}
}

func TestRandom_AllBackendsUsed(t *testing.T) {
	backends := []*backend.Backend{
		backend.New("http://backend1", "", 1),
		backend.New("http://backend2", "", 1),
		backend.New("http://backend3", "", 1),
	}

	r := NewRandom(backends)

	counts := make(map[string]int)
	requests := 300

	for i := 0; i < requests; i++ {
		b, err := r.NextBackend("")
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		counts[b.URL]++
	}

	// All backends should be used at least once in 300 requests
	for _, backend := range backends {
		if counts[backend.URL] == 0 {
			t.Errorf("Backend %s was never selected", backend.URL)
		}
	}

	// With random distribution, each should get roughly 100 requests (allow wide variance)
	for url, count := range counts {
		if count < 30 || count > 170 {
			t.Errorf("Backend %s: distribution suspicious: %d (expected ~100)", url, count)
		}
	}
}

func TestRandom_Concurrent(t *testing.T) {
	backends := []*backend.Backend{
		backend.New("http://backend1", "", 1),
		backend.New("http://backend2", "", 1),
		backend.New("http://backend3", "", 1),
	}

	r := NewRandom(backends)

	var wg sync.WaitGroup
	requests := 300

	for i := 0; i < requests; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := r.NextBackend("")
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
		}()
	}

	wg.Wait()
}

func TestBackend_ConnectionCounting(t *testing.T) {
	b := backend.New("http://test", "", 1)

	if b.GetActiveConnections() != 0 {
		t.Errorf("Expected 0 initial connections, got %d", b.GetActiveConnections())
	}

	b.IncrementConnections()
	if b.GetActiveConnections() != 1 {
		t.Errorf("Expected 1 connection after increment, got %d", b.GetActiveConnections())
	}

	b.IncrementConnections()
	b.IncrementConnections()
	if b.GetActiveConnections() != 3 {
		t.Errorf("Expected 3 connections, got %d", b.GetActiveConnections())
	}

	b.DecrementConnections()
	if b.GetActiveConnections() != 2 {
		t.Errorf("Expected 2 connections after decrement, got %d", b.GetActiveConnections())
	}
}

func TestBackend_ConcurrentConnectionCounting(t *testing.T) {
	b := backend.New("http://test", "", 1)

	var wg sync.WaitGroup
	iterations := 1000

	// Increment concurrently
	for i := 0; i < iterations; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			b.IncrementConnections()
		}()
	}
	wg.Wait()

	if b.GetActiveConnections() != int64(iterations) {
		t.Errorf("Expected %d connections, got %d", iterations, b.GetActiveConnections())
	}

	// Decrement concurrently
	for i := 0; i < iterations; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			b.DecrementConnections()
		}()
	}
	wg.Wait()

	if b.GetActiveConnections() != 0 {
		t.Errorf("Expected 0 connections after all decrements, got %d", b.GetActiveConnections())
	}
}

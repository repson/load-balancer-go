package backend

import (
	"sync"
	"testing"
)

// Helper functions for cleaner assertions
func assertEqual(t *testing.T, got, expected interface{}, msgAndArgs ...interface{}) {
	t.Helper()
	if got != expected {
		if len(msgAndArgs) > 0 {
			t.Errorf("%v: expected %v, got %v", msgAndArgs[0], expected, got)
		} else {
			t.Errorf("Expected %v, got %v", expected, got)
		}
	}
}

func assertNotEqual(t *testing.T, got, notExpected interface{}, msgAndArgs ...interface{}) {
	t.Helper()
	if got == notExpected {
		if len(msgAndArgs) > 0 {
			t.Errorf("%v: expected not to equal %v", msgAndArgs[0], notExpected)
		} else {
			t.Errorf("Expected not to equal %v", notExpected)
		}
	}
}

func TestNew(t *testing.T) {
	tests := []struct {
		name           string
		url            string
		address        string
		weight         int
		expectedWeight int
	}{
		{
			name:           "HTTP backend with valid weight",
			url:            "http://localhost:8080",
			address:        "",
			weight:         5,
			expectedWeight: 5,
		},
		{
			name:           "TCP backend with valid weight",
			url:            "",
			address:        "localhost:9090",
			weight:         3,
			expectedWeight: 3,
		},
		{
			name:           "Zero weight should default to 1",
			url:            "http://localhost:8080",
			address:        "",
			weight:         0,
			expectedWeight: 1,
		},
		{
			name:           "Negative weight should default to 1",
			url:            "http://localhost:8080",
			address:        "",
			weight:         -5,
			expectedWeight: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := New(tt.url, tt.address, tt.weight)

			assertEqual(t, b.URL, tt.url, "URL")
			assertEqual(t, b.Address, tt.address, "Address")
			assertEqual(t, b.Weight, tt.expectedWeight, "Weight")
			assertEqual(t, b.GetActiveConnections(), int64(0), "Initial connections")
		})
	}
}

func TestBackend_IncrementConnections(t *testing.T) {
	b := New("http://test", "", 1)

	assertEqual(t, b.GetActiveConnections(), int64(0), "Initial connections")

	b.IncrementConnections()
	assertEqual(t, b.GetActiveConnections(), int64(1), "After first increment")

	b.IncrementConnections()
	b.IncrementConnections()
	assertEqual(t, b.GetActiveConnections(), int64(3), "After three increments")
}

func TestBackend_DecrementConnections(t *testing.T) {
	b := New("http://test", "", 1)

	b.IncrementConnections()
	b.IncrementConnections()
	b.IncrementConnections()

	b.DecrementConnections()
	assertEqual(t, b.GetActiveConnections(), int64(2), "After one decrement")

	b.DecrementConnections()
	b.DecrementConnections()
	assertEqual(t, b.GetActiveConnections(), int64(0), "After all decrements")
}

func TestBackend_ConcurrentConnections(t *testing.T) {
	b := New("http://test", "", 1)

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

	assertEqual(t, b.GetActiveConnections(), int64(iterations), "After concurrent increments")

	// Decrement concurrently
	for i := 0; i < iterations; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			b.DecrementConnections()
		}()
	}
	wg.Wait()

	assertEqual(t, b.GetActiveConnections(), int64(0), "After concurrent decrements")
}

func TestBackend_String(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		address  string
		expected string
	}{
		{
			name:     "HTTP backend returns URL",
			url:      "http://localhost:8080",
			address:  "",
			expected: "http://localhost:8080",
		},
		{
			name:     "TCP backend returns address",
			url:      "",
			address:  "localhost:9090",
			expected: "localhost:9090",
		},
		{
			name:     "Both URL and address prefers URL",
			url:      "http://localhost:8080",
			address:  "localhost:9090",
			expected: "http://localhost:8080",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := New(tt.url, tt.address, 1)
			result := b.String()

			assertEqual(t, result, tt.expected, "String representation")
		})
	}
}

func TestBackend_GetActiveConnections(t *testing.T) {
	b := New("http://test", "", 1)

	// Multiple reads should be safe
	for i := 0; i < 10; i++ {
		if b.GetActiveConnections() != 0 {
			t.Errorf("Expected 0 connections, got %d", b.GetActiveConnections())
		}
	}

	b.IncrementConnections()

	// Multiple concurrent reads
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if b.GetActiveConnections() != 1 {
				t.Errorf("Expected 1 connection, got %d", b.GetActiveConnections())
			}
		}()
	}
	wg.Wait()
}

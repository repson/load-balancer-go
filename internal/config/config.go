package config

import "time"

// Config represents the main configuration structure
type Config struct {
	LogLevel    string       `yaml:"log_level"`
	MaxRetries  int          `yaml:"max_retries"`
	RetryDelay  string       `yaml:"retry_delay"`
	HTTP        *HTTPConfig  `yaml:"http"`
	TCP         *TCPConfig   `yaml:"tcp"`
	retryDelay  time.Duration // parsed retry delay
}

// HTTPConfig represents HTTP load balancer configuration
type HTTPConfig struct {
	Enabled   bool              `yaml:"enabled"`
	Listen    string            `yaml:"listen"`
	Algorithm string            `yaml:"algorithm"`
	Backends  []BackendConfig   `yaml:"backends"`
}

// TCPConfig represents TCP load balancer configuration
type TCPConfig struct {
	Enabled   bool              `yaml:"enabled"`
	Listen    string            `yaml:"listen"`
	Algorithm string            `yaml:"algorithm"`
	Backends  []BackendConfig   `yaml:"backends"`
}

// BackendConfig represents a backend server configuration
type BackendConfig struct {
	URL     string `yaml:"url"`     // For HTTP backends
	Address string `yaml:"address"` // For TCP backends
	Weight  int    `yaml:"weight"`
}

// GetRetryDelay returns the parsed retry delay duration
func (c *Config) GetRetryDelay() time.Duration {
	return c.retryDelay
}

// SetRetryDelay sets the parsed retry delay duration
func (c *Config) SetRetryDelay(d time.Duration) {
	c.retryDelay = d
}

// DefaultConfig returns a default configuration
func DefaultConfig() *Config {
	return &Config{
		LogLevel:   "info",
		MaxRetries: 3,
		RetryDelay: "100ms",
		HTTP: &HTTPConfig{
			Enabled:   true,
			Listen:    ":8080",
			Algorithm: "round-robin",
			Backends: []BackendConfig{
				{URL: "http://localhost:3001", Weight: 1},
				{URL: "http://localhost:3002", Weight: 1},
				{URL: "http://localhost:3003", Weight: 1},
			},
		},
		TCP: &TCPConfig{
			Enabled:   true,
			Listen:    ":9090",
			Algorithm: "round-robin",
			Backends: []BackendConfig{
				{Address: "localhost:4001", Weight: 1},
				{Address: "localhost:4002", Weight: 1},
			},
		},
	}
}

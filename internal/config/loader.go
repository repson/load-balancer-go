package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// Load loads configuration from a YAML file
func Load(filename string) (*Config, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Parse retry delay
	if cfg.RetryDelay != "" {
		delay, err := time.ParseDuration(cfg.RetryDelay)
		if err != nil {
			return nil, fmt.Errorf("invalid retry_delay format: %w", err)
		}
		cfg.SetRetryDelay(delay)
	}

	// Validate configuration
	if err := validate(&cfg); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return &cfg, nil
}

// validate validates the configuration
func validate(cfg *Config) error {
	// Validate log level
	validLevels := map[string]bool{
		"debug": true,
		"info":  true,
		"warn":  true,
		"error": true,
	}
	if !validLevels[cfg.LogLevel] {
		return fmt.Errorf("invalid log_level: %s (must be debug, info, warn, or error)", cfg.LogLevel)
	}

	// Validate max retries
	if cfg.MaxRetries < 0 {
		return fmt.Errorf("max_retries must be >= 0")
	}

	// Validate HTTP config
	if cfg.HTTP != nil && cfg.HTTP.Enabled {
		if cfg.HTTP.Listen == "" {
			return fmt.Errorf("http.listen cannot be empty")
		}
		if err := validateAlgorithm(cfg.HTTP.Algorithm); err != nil {
			return fmt.Errorf("http: %w", err)
		}
		if len(cfg.HTTP.Backends) == 0 {
			return fmt.Errorf("http: at least one backend is required")
		}
		for i, backend := range cfg.HTTP.Backends {
			if backend.URL == "" {
				return fmt.Errorf("http: backend %d: url is required", i)
			}
			if backend.Weight <= 0 {
				return fmt.Errorf("http: backend %d: weight must be > 0", i)
			}
		}
	}

	// Validate TCP config
	if cfg.TCP != nil && cfg.TCP.Enabled {
		if cfg.TCP.Listen == "" {
			return fmt.Errorf("tcp.listen cannot be empty")
		}
		if err := validateAlgorithm(cfg.TCP.Algorithm); err != nil {
			return fmt.Errorf("tcp: %w", err)
		}
		if len(cfg.TCP.Backends) == 0 {
			return fmt.Errorf("tcp: at least one backend is required")
		}
		for i, backend := range cfg.TCP.Backends {
			if backend.Address == "" {
				return fmt.Errorf("tcp: backend %d: address is required", i)
			}
			if backend.Weight <= 0 {
				return fmt.Errorf("tcp: backend %d: weight must be > 0", i)
			}
		}
	}

	// At least one protocol must be enabled
	if (cfg.HTTP == nil || !cfg.HTTP.Enabled) && (cfg.TCP == nil || !cfg.TCP.Enabled) {
		return fmt.Errorf("at least one of http or tcp must be enabled")
	}

	return nil
}

// validateAlgorithm validates the load balancing algorithm
func validateAlgorithm(algorithm string) error {
	validAlgorithms := map[string]bool{
		"round-robin":      true,
		"least-connections": true,
		"weighted":         true,
		"ip-hash":          true,
		"random":           true,
	}
	if !validAlgorithms[algorithm] {
		return fmt.Errorf("invalid algorithm: %s (must be round-robin, least-connections, weighted, ip-hash, or random)", algorithm)
	}
	return nil
}

package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.LogLevel != "info" {
		t.Errorf("Expected log level 'info', got %s", cfg.LogLevel)
	}

	if cfg.MaxRetries != 3 {
		t.Errorf("Expected max retries 3, got %d", cfg.MaxRetries)
	}

	if cfg.RetryDelay != "100ms" {
		t.Errorf("Expected retry delay '100ms', got %s", cfg.RetryDelay)
	}

	if cfg.HTTP == nil {
		t.Fatal("Expected HTTP config to be non-nil")
	}

	if !cfg.HTTP.Enabled {
		t.Error("Expected HTTP to be enabled")
	}

	if cfg.HTTP.Listen != ":8080" {
		t.Errorf("Expected HTTP listen ':8080', got %s", cfg.HTTP.Listen)
	}

	if cfg.HTTP.Algorithm != "round-robin" {
		t.Errorf("Expected HTTP algorithm 'round-robin', got %s", cfg.HTTP.Algorithm)
	}

	if len(cfg.HTTP.Backends) != 3 {
		t.Errorf("Expected 3 HTTP backends, got %d", len(cfg.HTTP.Backends))
	}

	if cfg.TCP == nil {
		t.Fatal("Expected TCP config to be non-nil")
	}

	if !cfg.TCP.Enabled {
		t.Error("Expected TCP to be enabled")
	}

	if cfg.TCP.Listen != ":9090" {
		t.Errorf("Expected TCP listen ':9090', got %s", cfg.TCP.Listen)
	}

	if len(cfg.TCP.Backends) != 2 {
		t.Errorf("Expected 2 TCP backends, got %d", len(cfg.TCP.Backends))
	}
}

func TestConfig_GetRetryDelay(t *testing.T) {
	cfg := &Config{}
	delay := 500 * time.Millisecond
	cfg.SetRetryDelay(delay)

	if cfg.GetRetryDelay() != delay {
		t.Errorf("Expected delay %v, got %v", delay, cfg.GetRetryDelay())
	}
}

func TestConfig_SetRetryDelay(t *testing.T) {
	cfg := &Config{}

	delays := []time.Duration{
		100 * time.Millisecond,
		1 * time.Second,
		5 * time.Second,
	}

	for _, delay := range delays {
		cfg.SetRetryDelay(delay)
		if cfg.GetRetryDelay() != delay {
			t.Errorf("Expected delay %v, got %v", delay, cfg.GetRetryDelay())
		}
	}
}

func TestLoad_ValidConfig(t *testing.T) {
	// Create a temporary config file
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")

	content := `log_level: debug
max_retries: 5
retry_delay: 200ms
http:
  enabled: true
  listen: ":8081"
  algorithm: least-connections
  backends:
    - url: http://localhost:3001
      weight: 2
    - url: http://localhost:3002
      weight: 1
tcp:
  enabled: false
  listen: ":9091"
  algorithm: round-robin
  backends:
    - address: localhost:4001
      weight: 1
`

	err := os.WriteFile(configFile, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	cfg, err := Load(configFile)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	if cfg.LogLevel != "debug" {
		t.Errorf("Expected log level 'debug', got %s", cfg.LogLevel)
	}

	if cfg.MaxRetries != 5 {
		t.Errorf("Expected max retries 5, got %d", cfg.MaxRetries)
	}

	if cfg.GetRetryDelay() != 200*time.Millisecond {
		t.Errorf("Expected retry delay 200ms, got %v", cfg.GetRetryDelay())
	}

	if cfg.HTTP.Listen != ":8081" {
		t.Errorf("Expected HTTP listen ':8081', got %s", cfg.HTTP.Listen)
	}

	if cfg.HTTP.Algorithm != "least-connections" {
		t.Errorf("Expected algorithm 'least-connections', got %s", cfg.HTTP.Algorithm)
	}

	if len(cfg.HTTP.Backends) != 2 {
		t.Errorf("Expected 2 backends, got %d", len(cfg.HTTP.Backends))
	}

	if cfg.HTTP.Backends[0].Weight != 2 {
		t.Errorf("Expected first backend weight 2, got %d", cfg.HTTP.Backends[0].Weight)
	}
}

func TestLoad_FileNotFound(t *testing.T) {
	_, err := Load("/nonexistent/config.yaml")
	if err == nil {
		t.Error("Expected error for nonexistent file, got nil")
	}
}

func TestLoad_InvalidYAML(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "invalid.yaml")

	content := `this is not: valid: yaml: content`
	err := os.WriteFile(configFile, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	_, err = Load(configFile)
	if err == nil {
		t.Error("Expected error for invalid YAML, got nil")
	}
}

func TestLoad_InvalidRetryDelay(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")

	content := `log_level: info
max_retries: 3
retry_delay: invalid
http:
  enabled: true
  listen: ":8080"
  algorithm: round-robin
  backends:
    - url: http://localhost:3001
      weight: 1
`

	err := os.WriteFile(configFile, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	_, err = Load(configFile)
	if err == nil {
		t.Error("Expected error for invalid retry_delay, got nil")
	}
}

func TestValidate_InvalidLogLevel(t *testing.T) {
	cfg := &Config{
		LogLevel:   "invalid",
		MaxRetries: 3,
		HTTP: &HTTPConfig{
			Enabled:   true,
			Listen:    ":8080",
			Algorithm: "round-robin",
			Backends: []BackendConfig{
				{URL: "http://localhost:3001", Weight: 1},
			},
		},
	}

	err := validate(cfg)
	if err == nil {
		t.Error("Expected error for invalid log level, got nil")
	}
}

func TestValidate_NegativeMaxRetries(t *testing.T) {
	cfg := &Config{
		LogLevel:   "info",
		MaxRetries: -1,
		HTTP: &HTTPConfig{
			Enabled:   true,
			Listen:    ":8080",
			Algorithm: "round-robin",
			Backends: []BackendConfig{
				{URL: "http://localhost:3001", Weight: 1},
			},
		},
	}

	err := validate(cfg)
	if err == nil {
		t.Error("Expected error for negative max_retries, got nil")
	}
}

func TestValidate_HTTPEmptyListen(t *testing.T) {
	cfg := &Config{
		LogLevel:   "info",
		MaxRetries: 3,
		HTTP: &HTTPConfig{
			Enabled:   true,
			Listen:    "",
			Algorithm: "round-robin",
			Backends: []BackendConfig{
				{URL: "http://localhost:3001", Weight: 1},
			},
		},
	}

	err := validate(cfg)
	if err == nil {
		t.Error("Expected error for empty HTTP listen, got nil")
	}
}

func TestValidate_HTTPInvalidAlgorithm(t *testing.T) {
	cfg := &Config{
		LogLevel:   "info",
		MaxRetries: 3,
		HTTP: &HTTPConfig{
			Enabled:   true,
			Listen:    ":8080",
			Algorithm: "invalid-algorithm",
			Backends: []BackendConfig{
				{URL: "http://localhost:3001", Weight: 1},
			},
		},
	}

	err := validate(cfg)
	if err == nil {
		t.Error("Expected error for invalid HTTP algorithm, got nil")
	}
}

func TestValidate_HTTPNoBackends(t *testing.T) {
	cfg := &Config{
		LogLevel:   "info",
		MaxRetries: 3,
		HTTP: &HTTPConfig{
			Enabled:   true,
			Listen:    ":8080",
			Algorithm: "round-robin",
			Backends:  []BackendConfig{},
		},
	}

	err := validate(cfg)
	if err == nil {
		t.Error("Expected error for no HTTP backends, got nil")
	}
}

func TestValidate_HTTPBackendEmptyURL(t *testing.T) {
	cfg := &Config{
		LogLevel:   "info",
		MaxRetries: 3,
		HTTP: &HTTPConfig{
			Enabled:   true,
			Listen:    ":8080",
			Algorithm: "round-robin",
			Backends: []BackendConfig{
				{URL: "", Weight: 1},
			},
		},
	}

	err := validate(cfg)
	if err == nil {
		t.Error("Expected error for empty backend URL, got nil")
	}
}

func TestValidate_HTTPBackendInvalidWeight(t *testing.T) {
	cfg := &Config{
		LogLevel:   "info",
		MaxRetries: 3,
		HTTP: &HTTPConfig{
			Enabled:   true,
			Listen:    ":8080",
			Algorithm: "round-robin",
			Backends: []BackendConfig{
				{URL: "http://localhost:3001", Weight: 0},
			},
		},
	}

	err := validate(cfg)
	if err == nil {
		t.Error("Expected error for invalid backend weight, got nil")
	}
}

func TestValidate_TCPConfig(t *testing.T) {
	cfg := &Config{
		LogLevel:   "info",
		MaxRetries: 3,
		TCP: &TCPConfig{
			Enabled:   true,
			Listen:    ":9090",
			Algorithm: "round-robin",
			Backends: []BackendConfig{
				{Address: "localhost:4001", Weight: 1},
			},
		},
	}

	err := validate(cfg)
	if err != nil {
		t.Errorf("Unexpected error for valid TCP config: %v", err)
	}
}

func TestValidate_TCPBackendEmptyAddress(t *testing.T) {
	cfg := &Config{
		LogLevel:   "info",
		MaxRetries: 3,
		TCP: &TCPConfig{
			Enabled:   true,
			Listen:    ":9090",
			Algorithm: "round-robin",
			Backends: []BackendConfig{
				{Address: "", Weight: 1},
			},
		},
	}

	err := validate(cfg)
	if err == nil {
		t.Error("Expected error for empty TCP backend address, got nil")
	}
}

func TestValidate_NoProtocolsEnabled(t *testing.T) {
	cfg := &Config{
		LogLevel:   "info",
		MaxRetries: 3,
		HTTP: &HTTPConfig{
			Enabled: false,
		},
		TCP: &TCPConfig{
			Enabled: false,
		},
	}

	err := validate(cfg)
	if err == nil {
		t.Error("Expected error when no protocols enabled, got nil")
	}
}

func TestConfig_GetDialTimeout_Default(t *testing.T) {
	cfg := &TCPConfig{}
	// zero value should return the 5s default
	if got := cfg.GetDialTimeout(); got != 5*time.Second {
		t.Errorf("Expected default dial timeout 5s, got %v", got)
	}
}

func TestConfig_GetSetDialTimeout(t *testing.T) {
	cfg := &TCPConfig{}
	d := 2 * time.Second
	cfg.SetDialTimeout(d)
	if got := cfg.GetDialTimeout(); got != d {
		t.Errorf("Expected dial timeout %v, got %v", d, got)
	}
}

func TestLoad_WithDialTimeout(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")

	content := `log_level: info
max_retries: 3
retry_delay: 100ms
tcp:
  enabled: true
  listen: ":9090"
  algorithm: round-robin
  dial_timeout: 3s
  backends:
    - address: localhost:4001
      weight: 1
`
	if err := os.WriteFile(configFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	cfg, err := Load(configFile)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if got := cfg.TCP.GetDialTimeout(); got != 3*time.Second {
		t.Errorf("Expected dial_timeout 3s, got %v", got)
	}
}

func TestLoad_InvalidDialTimeout(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")

	content := `log_level: info
max_retries: 3
retry_delay: 100ms
tcp:
  enabled: true
  listen: ":9090"
  algorithm: round-robin
  dial_timeout: notaduration
  backends:
    - address: localhost:4001
      weight: 1
`
	if err := os.WriteFile(configFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	_, err := Load(configFile)
	if err == nil {
		t.Error("Expected error for invalid tcp.dial_timeout, got nil")
	}
}

func TestValidate_TCPEmptyListen(t *testing.T) {
	cfg := &Config{
		LogLevel:   "info",
		MaxRetries: 3,
		TCP: &TCPConfig{
			Enabled:   true,
			Listen:    "",
			Algorithm: "round-robin",
			Backends:  []BackendConfig{{Address: "localhost:4001", Weight: 1}},
		},
	}
	if err := validate(cfg); err == nil {
		t.Error("Expected error for empty tcp.listen, got nil")
	}
}

func TestValidate_TCPInvalidAlgorithm(t *testing.T) {
	cfg := &Config{
		LogLevel:   "info",
		MaxRetries: 3,
		TCP: &TCPConfig{
			Enabled:   true,
			Listen:    ":9090",
			Algorithm: "invalid",
			Backends:  []BackendConfig{{Address: "localhost:4001", Weight: 1}},
		},
	}
	if err := validate(cfg); err == nil {
		t.Error("Expected error for invalid tcp algorithm, got nil")
	}
}

func TestValidate_TCPNoBackends(t *testing.T) {
	cfg := &Config{
		LogLevel:   "info",
		MaxRetries: 3,
		TCP: &TCPConfig{
			Enabled:   true,
			Listen:    ":9090",
			Algorithm: "round-robin",
			Backends:  []BackendConfig{},
		},
	}
	if err := validate(cfg); err == nil {
		t.Error("Expected error for no TCP backends, got nil")
	}
}

func TestValidate_TCPBackendInvalidWeight(t *testing.T) {
	cfg := &Config{
		LogLevel:   "info",
		MaxRetries: 3,
		TCP: &TCPConfig{
			Enabled:   true,
			Listen:    ":9090",
			Algorithm: "round-robin",
			Backends:  []BackendConfig{{Address: "localhost:4001", Weight: 0}},
		},
	}
	if err := validate(cfg); err == nil {
		t.Error("Expected error for TCP backend weight 0, got nil")
	}
}

func TestValidateAlgorithm(t *testing.T) {
	validAlgorithms := []string{
		"round-robin",
		"least-connections",
		"weighted",
		"ip-hash",
		"random",
	}

	for _, algo := range validAlgorithms {
		err := validateAlgorithm(algo)
		if err != nil {
			t.Errorf("Expected no error for valid algorithm '%s', got %v", algo, err)
		}
	}

	invalidAlgorithms := []string{
		"invalid",
		"round_robin",
		"leastconnections",
		"",
	}

	for _, algo := range invalidAlgorithms {
		err := validateAlgorithm(algo)
		if err == nil {
			t.Errorf("Expected error for invalid algorithm '%s', got nil", algo)
		}
	}
}

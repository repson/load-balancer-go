# Load Balancer Go

A educational load balancer implementation in Go featuring multiple balancing algorithms, HTTP/HTTPS reverse proxy, and TCP proxying capabilities. Built for learning and demonstration purposes.

## Features

- **5 Load Balancing Algorithms**
  - Round Robin - Sequential distribution across backends
  - Least Connections - Routes to backend with fewest active connections
  - Weighted Round Robin - Distribution based on backend weights
  - IP Hash - Consistent routing based on client IP
  - Random - Random backend selection

- **Dual Protocol Support**
  - HTTP/HTTPS reverse proxy
  - TCP transparent proxy

- **Reliability**
  - Automatic retry with configurable attempts
  - Fallback to next backend on failure
  - Thread-safe connection counting

- **Configuration**
  - YAML configuration file support
  - Built-in sensible defaults
  - CLI flag for custom config path

- **Observability**
  - Structured logging with slog
  - Configurable log levels (debug, info, warn, error)
  - Request/response tracking

## Quick Start

### Prerequisites

- Go 1.21 or higher
- (Optional) Docker for containerized deployment

### Installation

```bash
# Clone the repository
git clone https://github.com/repson/load-balancer-go.git
cd load-balancer-go

# Download dependencies
go mod download

# Build the load balancer
go build -o loadbalancer cmd/loadbalancer/main.go
```

### Running with Default Configuration

```bash
# Start the load balancer with built-in defaults
./loadbalancer

# Or using go run
go run cmd/loadbalancer/main.go
```

**Default Configuration:**
- HTTP proxy on `:8080` with Round Robin
- TCP proxy on `:9090` with Round Robin
- Backends: localhost:3001, 3002, 3003 (HTTP) and 4001, 4002 (TCP)

### Running with Custom Configuration

```bash
# Use a custom YAML config file
./loadbalancer -config examples/config.yaml
```

## Configuration

### YAML Configuration File

```yaml
# General settings
log_level: "info"        # Options: debug, info, warn, error
max_retries: 3           # Maximum retry attempts
retry_delay: "100ms"     # Delay between retries

# HTTP Load Balancer
http:
  enabled: true
  listen: ":8080"
  algorithm: "round-robin"  # round-robin, least-connections, weighted, ip-hash, random
  backends:
    - url: "http://localhost:3001"
      weight: 1
    - url: "http://localhost:3002"
      weight: 1
    - url: "http://localhost:3003"
      weight: 2

# TCP Load Balancer
tcp:
  enabled: true
  listen: ":9090"
  algorithm: "least-connections"
  dial_timeout: "5s"            # Timeout for connecting to a backend (default: 5s)
  backends:
    - address: "localhost:4001"
      weight: 1
    - address: "localhost:4002"
      weight: 1
```

See `examples/config.yaml` for a complete example.

### Configuration Options

#### Global Settings

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `log_level` | string | `info` | Logging verbosity level |
| `max_retries` | int | `3` | Number of retry attempts |
| `retry_delay` | duration | `100ms` | Delay between retries |

#### HTTP Configuration

| Option | Type | Required | Description |
|--------|------|----------|-------------|
| `enabled` | bool | No | Enable HTTP proxy (default: true) |
| `listen` | string | Yes | Listen address (e.g., `:8080`) |
| `algorithm` | string | Yes | Load balancing algorithm |
| `backends` | array | Yes | List of backend servers |
| `backends[].url` | string | Yes | Backend URL |
| `backends[].weight` | int | Yes | Backend weight (for weighted algorithm) |

#### TCP Configuration

| Option | Type | Required | Description |
|--------|------|----------|-------------|
| `enabled` | bool | No | Enable TCP proxy (default: true) |
| `listen` | string | Yes | Listen address (e.g., `:9090`) |
| `algorithm` | string | Yes | Load balancing algorithm |
| `dial_timeout` | duration | No | Backend connection timeout (default: `5s`) |
| `backends` | array | Yes | List of backend servers |
| `backends[].address` | string | Yes | Backend address (host:port) |
| `backends[].weight` | int | Yes | Backend weight (for weighted algorithm) |

## Load Balancing Algorithms

### Round Robin

Distributes requests sequentially across all backends in a circular fashion.

**Use case:** Equal load distribution when all backends have similar capacity.

```yaml
algorithm: "round-robin"
```

### Least Connections

Routes requests to the backend with the fewest active connections. Thread-safe implementation using atomic operations.

**Use case:** Long-lived connections or variable request processing times.

```yaml
algorithm: "least-connections"
```

### Weighted Round Robin

Distributes requests based on backend weights. Backends with higher weights receive proportionally more requests.

**Use case:** Backends with different capacities.

```yaml
algorithm: "weighted"
backends:
  - url: "http://server1"
    weight: 1    # Receives 25% of traffic
  - url: "http://server2"
    weight: 3    # Receives 75% of traffic
```

### IP Hash

Uses a hash of the client IP address to determine the backend. Ensures the same client always reaches the same backend.

**Use case:** Session affinity without sticky sessions.

```yaml
algorithm: "ip-hash"
```

### Random

Randomly selects a backend for each request.

**Use case:** Simple load distribution without state.

```yaml
algorithm: "random"
```

## Testing

### Running Test Servers

Start the mock HTTP servers:

```bash
go run examples/test-servers/http/main.go
```

This starts three HTTP servers on ports 3001, 3002, and 3003.

Start the mock TCP servers:

```bash
go run examples/test-servers/tcp/main.go
```

This starts two TCP echo servers on ports 4001 and 4002.

### Testing HTTP Load Balancing

```bash
# Start the load balancer
./loadbalancer

# In another terminal, send requests
curl http://localhost:8080/
curl http://localhost:8080/
curl http://localhost:8080/

# Each request should be routed to a different backend
```

### Testing TCP Load Balancing

```bash
# Connect to the TCP proxy
telnet localhost 9090

# Type messages and see them echoed back
# Type 'quit' to disconnect
```

### Running Unit Tests

```bash
# Run all tests
go test ./...

# Run tests with coverage
go test -cover ./...

# Run specific package tests
go test ./internal/balancer/...
```

### Running Integration Tests

```bash
# Run integration tests
go test ./tests/...
```

## Architecture

```
┌─────────────────────────────────────────────────────┐
│                  Load Balancer                      │
│                                                     │
│  ┌──────────────┐         ┌──────────────┐        │
│  │ HTTP Proxy   │         │  TCP Proxy   │        │
│  │  :8080       │         │   :9090      │        │
│  └──────┬───────┘         └──────┬───────┘        │
│         │                        │                 │
│         └────────┬───────────────┘                │
│                  │                                 │
│         ┌────────▼────────┐                       │
│         │    Balancer     │                       │
│         │   (Algorithm)   │                       │
│         └────────┬────────┘                       │
│                  │                                 │
│         ┌────────▼────────┐                       │
│         │    Backends     │                       │
│         │   (Servers)     │                       │
│         └─────────────────┘                       │
└─────────────────────────────────────────────────────┘
                    │
        ┌───────────┼───────────┐
        │           │           │
   ┌────▼───┐  ┌───▼────┐ ┌───▼────┐
   │Backend1│  │Backend2│ │Backend3│
   └────────┘  └────────┘ └────────┘
```

### Components

- **Config**: YAML configuration parser and validator
- **Backend**: Thread-safe backend server model
- **Balancer**: Interface and algorithm implementations
- **Proxy**: HTTP and TCP proxy implementations with retry logic
- **Logger**: Structured logging wrapper

## Project Structure

```
load-balancer-go/
├── cmd/
│   └── loadbalancer/
│       └── main.go           # Entry point
├── internal/
│   ├── config/               # Configuration management
│   ├── backend/              # Backend model
│   ├── balancer/             # Balancing algorithms
│   ├── proxy/                # HTTP and TCP proxies
│   └── logger/               # Logging utilities
├── examples/
│   ├── config.yaml           # Example configuration
│   └── test-servers/         # Mock servers for testing
├── tests/                    # Integration tests
├── go.mod
└── README.md
```

## Performance Considerations

- **Thread Safety**: All connection counters use atomic operations; the Random balancer protects its RNG with a mutex
- **Goroutines**: Each TCP connection is handled in a separate goroutine; both copy directions are waited on before cleanup to prevent leaks
- **Connection Pooling**: HTTP proxy uses Go's built-in connection pooling
- **Retry Safety**: HTTP retries buffer each attempt internally so partial responses are never flushed to the client on failure
- **Retry Logic**: Configurable retry attempts prevent cascading failures

## Limitations (MVP Scope)

This is a demonstration project. The following features are NOT included:

- Health checks for backends
- Hot reload of configuration
- Metrics/Prometheus integration
- Sticky sessions
- Rate limiting
- Circuit breaker
- TLS/HTTPS termination
- WebSocket support

## Contributing

This is an educational project. Feel free to fork and experiment!

## License

MIT License

## Acknowledgments

- Built with Go's standard library
- Uses `gopkg.in/yaml.v3` for YAML parsing
- Inspired by production load balancers like HAProxy and Nginx

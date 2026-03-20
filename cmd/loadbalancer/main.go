package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/isaac/load-balancer-go/internal/backend"
	"github.com/isaac/load-balancer-go/internal/balancer"
	"github.com/isaac/load-balancer-go/internal/config"
	"github.com/isaac/load-balancer-go/internal/logger"
	"github.com/isaac/load-balancer-go/internal/proxy"
)

func main() {
	configFile := flag.String("config", "examples/config.yaml", "path to config file")
	flag.Parse()

	cfg, err := config.Load(*configFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load config: %v\n", err)
		os.Exit(1)
	}

	logger.Init(cfg.LogLevel)

	errCh := make(chan error, 2)

	// Start HTTP proxy
	var httpServer *http.Server
	if cfg.HTTP != nil && cfg.HTTP.Enabled {
		bal, err := buildBalancer(cfg.HTTP.Algorithm, cfg.HTTP.Backends, true)
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to build HTTP balancer: %v\n", err)
			os.Exit(1)
		}

		httpProxy := proxy.NewHTTPProxy(bal, cfg.MaxRetries, cfg.GetRetryDelay())
		httpServer = &http.Server{
			Addr:    cfg.HTTP.Listen,
			Handler: httpProxy,
		}

		go func() {
			logger.Info("HTTP proxy starting", "address", cfg.HTTP.Listen, "algorithm", cfg.HTTP.Algorithm)
			if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				errCh <- fmt.Errorf("HTTP proxy: %w", err)
			}
		}()
	}

	// Start TCP proxy
	if cfg.TCP != nil && cfg.TCP.Enabled {
		bal, err := buildBalancer(cfg.TCP.Algorithm, cfg.TCP.Backends, false)
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to build TCP balancer: %v\n", err)
			os.Exit(1)
		}

		tcpProxy := proxy.NewTCPProxy(bal, cfg.MaxRetries, cfg.GetRetryDelay())
		go func() {
			logger.Info("TCP proxy starting", "address", cfg.TCP.Listen, "algorithm", cfg.TCP.Algorithm)
			if err := tcpProxy.Serve(cfg.TCP.Listen); err != nil {
				errCh <- fmt.Errorf("TCP proxy: %w", err)
			}
		}()
	}

	// Wait for signal or fatal error
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-errCh:
		logger.Error("Fatal error", "error", err)
		os.Exit(1)
	case sig := <-quit:
		logger.Info("Shutting down", "signal", sig)
	}

	// Graceful shutdown of HTTP server
	if httpServer != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := httpServer.Shutdown(ctx); err != nil {
			logger.Error("HTTP server shutdown error", "error", err)
		}
	}
}

// buildBalancer creates a Balancer from config, using URL for HTTP backends and Address for TCP.
func buildBalancer(algorithm string, cfgBackends []config.BackendConfig, isHTTP bool) (balancer.Balancer, error) {
	backends := make([]*backend.Backend, 0, len(cfgBackends))
	for _, b := range cfgBackends {
		backends = append(backends, backend.New(b.URL, b.Address, b.Weight))
	}

	switch algorithm {
	case "round-robin":
		return balancer.NewRoundRobin(backends), nil
	case "least-connections":
		return balancer.NewLeastConnections(backends), nil
	case "weighted":
		return balancer.NewWeightedRoundRobin(backends), nil
	case "ip-hash":
		return balancer.NewIPHash(backends), nil
	case "random":
		return balancer.NewRandom(backends), nil
	default:
		return nil, fmt.Errorf("unknown algorithm: %s", algorithm)
	}
}

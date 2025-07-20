package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/k8s-shard-controller/pkg/config"
	"github.com/k8s-shard-controller/pkg/controllers"
)

var (
	kubeconfig   = flag.String("kubeconfig", "", "Path to kubeconfig file")
	masterURL    = flag.String("master", "", "The address of the Kubernetes API server")
	namespace    = flag.String("namespace", "default", "Namespace to operate in")
	logLevel     = flag.String("log-level", "info", "Log level (debug, info, warn, error)")
	configFile   = flag.String("config", "", "Path to configuration file")
	metricsAddr  = flag.String("metrics-addr", ":8080", "The address the metric endpoint binds to")
	healthAddr   = flag.String("health-addr", ":8081", "The address the health endpoint binds to")
	leaderElect  = flag.Bool("leader-elect", false, "Enable leader election for controller manager")
	healthCheck  = flag.Bool("health-check", false, "Run health check and exit")
)

func main() {
	flag.Parse()

	// Handle health check mode
	if *healthCheck {
		// Simple health check - just exit with 0 if binary can run
		os.Exit(0)
	}

	// Setup logging
	opts := zap.Options{
		Development: *logLevel == "debug",
	}
	log.SetLogger(zap.New(zap.UseFlagOptions(&opts)))
	logger := log.Log.WithName("shard-controller")

	logger.Info("Starting Kubernetes Shard Controller", 
		"metricsAddr", *metricsAddr, 
		"healthAddr", *healthAddr, 
		"leaderElect", *leaderElect)

	// Load configuration
	cfg := config.DefaultConfig()
	cfg.KubeConfig = *kubeconfig
	cfg.MasterURL = *masterURL
	cfg.Namespace = *namespace
	cfg.LogLevel = *logLevel

	// Load config file if provided
	if *configFile != "" {
		logger.Info("Loading configuration from file", "configFile", *configFile)
		// TODO: Implement config file loading
	}

	// Create controller manager
	controllerManager, err := controllers.NewControllerManager(cfg)
	if err != nil {
		logger.Error(err, "Failed to create controller manager")
		os.Exit(1)
	}

	logger.Info("Controller manager created successfully")

	// Setup health endpoints
	setupHealthEndpoints(*healthAddr, controllerManager)

	// Setup signal handling
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigCh
		logger.Info("Received signal, shutting down", "signal", sig)
		cancel()
	}()

	// Start controller manager
	if err := controllerManager.Start(ctx); err != nil {
		logger.Error(err, "Failed to start controller manager")
		os.Exit(1)
	}

	logger.Info("Controller manager started successfully")

	// Wait for shutdown
	<-ctx.Done()

	// Stop controller manager
	if err := controllerManager.Stop(); err != nil {
		logger.Error(err, "Failed to stop controller manager gracefully")
	}

	logger.Info("Shard Controller stopped")
}

// setupHealthEndpoints sets up health and readiness endpoints
func setupHealthEndpoints(healthAddr string, controllerManager *controllers.ControllerManager) {
	mux := http.NewServeMux()
	
	// Health endpoint - checks if the service is alive
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		if controllerManager.IsRunning() {
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, "ok")
		} else {
			w.WriteHeader(http.StatusServiceUnavailable)
			fmt.Fprint(w, "not ready")
		}
	})
	
	// Readiness endpoint - checks if the service is ready to serve traffic
	mux.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
		if controllerManager.IsRunning() {
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, "ready")
		} else {
			w.WriteHeader(http.StatusServiceUnavailable)
			fmt.Fprint(w, "not ready")
		}
	})
	
	// Start health server in a goroutine
	go func() {
		logger := log.Log.WithName("health-server")
		logger.Info("Starting health server", "addr", healthAddr)
		
		server := &http.Server{
			Addr:    healthAddr,
			Handler: mux,
		}
		
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error(err, "Health server failed")
		}
	}()
}

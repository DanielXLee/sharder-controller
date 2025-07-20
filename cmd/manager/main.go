package main

import (
	"context"
	"flag"
	"os"
	"os/signal"
	"syscall"

	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/k8s-shard-controller/pkg/config"
	"github.com/k8s-shard-controller/pkg/controllers"
)

var (
	kubeconfig = flag.String("kubeconfig", "", "Path to kubeconfig file")
	masterURL  = flag.String("master", "", "The address of the Kubernetes API server")
	namespace  = flag.String("namespace", "default", "Namespace to operate in")
	logLevel   = flag.String("log-level", "info", "Log level (debug, info, warn, error)")
)

func main() {
	flag.Parse()

	// Setup logging
	opts := zap.Options{
		Development: *logLevel == "debug",
	}
	log.SetLogger(zap.New(zap.UseFlagOptions(&opts)))
	logger := log.Log.WithName("shard-controller")

	logger.Info("Starting Kubernetes Shard Controller")

	// Load configuration
	cfg := config.DefaultConfig()
	cfg.KubeConfig = *kubeconfig
	cfg.MasterURL = *masterURL
	cfg.Namespace = *namespace
	cfg.LogLevel = *logLevel

	// Create controller manager
	controllerManager, err := controllers.NewControllerManager(cfg)
	if err != nil {
		logger.Error(err, "Failed to create controller manager")
		os.Exit(1)
	}

	logger.Info("Controller manager created successfully")

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

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
	"github.com/k8s-shard-controller/pkg/worker"
)

var (
	kubeconfig = flag.String("kubeconfig", "", "Path to kubeconfig file")
	masterURL  = flag.String("master", "", "The address of the Kubernetes API server")
	namespace  = flag.String("namespace", "default", "Namespace to operate in")
	logLevel   = flag.String("log-level", "info", "Log level (debug, info, warn, error)")
	shardId    = flag.String("shard-id", "", "The ID of this shard")
)

func main() {
	flag.Parse()

	// Setup logging
	opts := zap.Options{
		Development: *logLevel == "debug",
	}
	log.SetLogger(zap.New(zap.UseFlagOptions(&opts)))
	logger := log.Log.WithName("shard-worker")

	logger.Info("Starting Kubernetes Shard Worker")

	// Load configuration
	cfg := config.DefaultConfig()
	cfg.KubeConfig = *kubeconfig
	cfg.MasterURL = *masterURL
	cfg.Namespace = *namespace
	cfg.LogLevel = *logLevel

	// Create worker shard
	workerShard, err := worker.NewWorkerShard(cfg, *shardId)
	if err != nil {
		logger.Error(err, "Failed to create worker shard")
		os.Exit(1)
	}

	logger.Info("Worker shard created successfully")

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

	// Start worker shard
	if err := workerShard.Start(ctx); err != nil {
		logger.Error(err, "Failed to start worker shard")
		os.Exit(1)
	}

	logger.Info("Worker shard started successfully")

	// Wait for shutdown
	<-ctx.Done()

	// Stop worker shard
	if err := workerShard.Stop(); err != nil {
		logger.Error(err, "Failed to stop worker shard gracefully")
	}

	logger.Info("Shard Worker stopped")
}

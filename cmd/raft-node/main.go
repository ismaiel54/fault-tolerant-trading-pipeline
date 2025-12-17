package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ismaiel54/fault-tolerant-trading-pipeline/internal/config"
	"github.com/ismaiel54/fault-tolerant-trading-pipeline/internal/logging"
	"github.com/ismaiel54/fault-tolerant-trading-pipeline/internal/observability"
	"github.com/ismaiel54/fault-tolerant-trading-pipeline/internal/rpc/raftnode"
	tradingv1 "github.com/ismaiel54/fault-tolerant-trading-pipeline/gen/proto/trading/v1"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

func main() {
	// Load configuration
	cfg := config.LoadConfig("raft-node")

	// Initialize logger
	logger, err := logging.NewLogger(cfg.ServiceName, cfg.LogLevel)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to initialize logger: %v\n", err)
		os.Exit(1)
	}
	defer logger.Sync()

	logger.Info("starting raft-node service",
		zap.Int("grpc_port", cfg.GRPCPort),
		zap.Int("http_port", cfg.HTTPPort),
	)

	// Create health checker
	healthChecker := observability.NewHealthChecker(logger)

	// Create gRPC server
	grpcServer := grpc.NewServer()
	healthChecker.RegisterGRPC(grpcServer)
	
	// Register RaftNodeService
	raftNodeServer := raftnode.NewServer(logger)
	tradingv1.RegisterRaftNodeServiceServer(grpcServer, raftNodeServer)

	// Start gRPC server
	grpcListener, err := net.Listen("tcp", cfg.GRPCAddr())
	if err != nil {
		logger.Fatal("failed to listen on gRPC port", zap.Error(err))
	}

	grpcErrCh := make(chan error, 1)
	go func() {
		logger.Info("gRPC server listening", zap.String("addr", cfg.GRPCAddr()))
		if err := grpcServer.Serve(grpcListener); err != nil {
			grpcErrCh <- err
		}
	}()

	// Start HTTP health server
	httpErrCh := make(chan error, 1)
	go func() {
		if err := healthChecker.StartHTTPServer(cfg.HTTPAddr()); err != nil && err != http.ErrServerClosed {
			httpErrCh <- err
		}
	}()

	// Wait for interrupt signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	select {
	case sig := <-sigCh:
		logger.Info("received shutdown signal", zap.String("signal", sig.String()))
	case err := <-grpcErrCh:
		logger.Error("gRPC server error", zap.Error(err))
	case err := <-httpErrCh:
		logger.Error("HTTP server error", zap.Error(err))
	}

	// Graceful shutdown
	logger.Info("shutting down gracefully...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Shutdown health checker
	if err := healthChecker.Shutdown(ctx); err != nil {
		logger.Error("error shutting down health checker", zap.Error(err))
	}

	// Shutdown gRPC server
	grpcServer.GracefulStop()

	logger.Info("raft-node service stopped")
}

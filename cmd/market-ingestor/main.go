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
	"github.com/ismaiel54/fault-tolerant-trading-pipeline/internal/rpc/streamprocessor"
	tradingv1 "github.com/ismaiel54/fault-tolerant-trading-pipeline/gen/proto/trading/v1"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

func main() {
	// Load configuration
	cfg := config.LoadConfig("market-ingestor")

	// Initialize logger
	logger, err := logging.NewLogger(cfg.ServiceName, cfg.LogLevel)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to initialize logger: %v\n", err)
		os.Exit(1)
	}
	defer logger.Sync()

	logger.Info("starting market-ingestor service",
		zap.Int("grpc_port", cfg.GRPCPort),
		zap.Int("http_port", cfg.HTTPPort),
		zap.String("stream_processor_addr", cfg.StreamProcessorGRPCAddr),
	)

	// Dial stream-processor client
	ctx := context.Background()
	streamProcessorClient, err := streamprocessor.Dial(ctx, cfg.StreamProcessorGRPCAddr, logger)
	if err != nil {
		logger.Fatal("failed to dial stream-processor", zap.Error(err))
	}
	defer streamProcessorClient.Close()

	// Create health checker
	healthChecker := observability.NewHealthChecker(logger)

	// Create gRPC server
	grpcServer := grpc.NewServer()
	healthChecker.RegisterGRPC(grpcServer)

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

	// Start tick sender
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	
	tickCounter := 0.0
	tickCh := make(chan struct{})
	go func() {
		for range ticker.C {
			tickCounter += 0.01
			tick := &tradingv1.Tick{
				Symbol:       "AAPL",
				Price:        150.0 + tickCounter,
				TsUnixMillis: time.Now().UnixMilli(),
			}
			
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			ack, err := streamProcessorClient.ProcessTick(ctx, tick)
			cancel()
			
			if err != nil {
				logger.Error("failed to process tick", zap.Error(err))
			} else {
				logger.Info("tick processed",
					zap.String("request_id", ack.RequestId),
					zap.String("message", ack.Message),
					zap.String("symbol", tick.Symbol),
					zap.Float64("price", tick.Price),
				)
			}
		}
		close(tickCh)
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

	// Stop ticker
	ticker.Stop()
	<-tickCh

	// Close stream-processor client
	if err := streamProcessorClient.Close(); err != nil {
		logger.Error("error closing stream-processor client", zap.Error(err))
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Shutdown health checker
	if err := healthChecker.Shutdown(shutdownCtx); err != nil {
		logger.Error("error shutting down health checker", zap.Error(err))
	}

	// Shutdown gRPC server
	grpcServer.GracefulStop()

	logger.Info("market-ingestor service stopped")
}

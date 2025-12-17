package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/ismaiel54/fault-tolerant-trading-pipeline/internal/config"
	"github.com/ismaiel54/fault-tolerant-trading-pipeline/internal/logging"
	"github.com/ismaiel54/fault-tolerant-trading-pipeline/internal/msg"
	"github.com/ismaiel54/fault-tolerant-trading-pipeline/internal/observability"
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
		zap.String("kafka_brokers", cfg.KafkaBrokers),
	)

	// Create health checker
	healthChecker := observability.NewHealthChecker(logger)

	// Create Kafka producer
	brokers := strings.Split(cfg.KafkaBrokers, ",")
	for i := range brokers {
		brokers[i] = strings.TrimSpace(brokers[i])
	}
	producer, err := msg.NewProducer(brokers, logger)
	if err != nil {
		logger.Fatal("failed to create kafka producer", zap.Error(err))
	}
	defer producer.Close()
	healthChecker.SetKafkaReady(true)

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

	// Start tick producer
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	tickCounter := 0.0
	tickCh := make(chan struct{})
	go func() {
		defer close(tickCh)
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				tickCounter += 0.01
				eventID := uuid.New().String()
				tickMsg := msg.TickMsg{
					EventID:      eventID,
					Symbol:       "AAPL",
					Price:        150.0 + tickCounter,
					TsUnixMillis: time.Now().UnixMilli(),
				}

				if err := producer.ProduceJSON(ctx, msg.TopicMarketTicks, tickMsg.Symbol, tickMsg); err != nil {
					logger.Error("failed to produce tick", zap.Error(err))
				} else {
					logger.Info("tick produced",
						zap.String("event_id", eventID),
						zap.String("symbol", tickMsg.Symbol),
						zap.Float64("price", tickMsg.Price),
					)
				}
			}
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

	// Stop ticker
	cancel()
	ticker.Stop()
	<-tickCh

	// Close producer
	producer.Close()

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

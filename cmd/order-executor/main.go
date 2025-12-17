package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/ismaiel54/fault-tolerant-trading-pipeline/internal/config"
	"github.com/ismaiel54/fault-tolerant-trading-pipeline/internal/idempotency"
	"github.com/ismaiel54/fault-tolerant-trading-pipeline/internal/logging"
	"github.com/ismaiel54/fault-tolerant-trading-pipeline/internal/msg"
	"github.com/ismaiel54/fault-tolerant-trading-pipeline/internal/observability"
	"github.com/ismaiel54/fault-tolerant-trading-pipeline/internal/rpc/orderexecutor"
	tradingv1 "github.com/ismaiel54/fault-tolerant-trading-pipeline/gen/proto/trading/v1"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

func main() {
	// Load configuration
	cfg := config.LoadConfig("order-executor")

	// Initialize logger
	logger, err := logging.NewLogger(cfg.ServiceName, cfg.LogLevel)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to initialize logger: %v\n", err)
		os.Exit(1)
	}
	defer logger.Sync()

	logger.Info("starting order-executor service",
		zap.Int("grpc_port", cfg.GRPCPort),
		zap.Int("http_port", cfg.HTTPPort),
		zap.String("kafka_brokers", cfg.KafkaBrokers),
		zap.String("data_dir", cfg.DataDir),
	)

	// Create data directory
	if err := os.MkdirAll(cfg.DataDir, 0755); err != nil {
		logger.Fatal("failed to create data directory", zap.Error(err))
	}

	// Open idempotency store
	dbPath := filepath.Join(cfg.DataDir, "orders.db")
	store, err := idempotency.Open(dbPath)
	if err != nil {
		logger.Fatal("failed to open idempotency store", zap.Error(err))
	}
	defer store.Close()

	logger.Info("idempotency store opened", zap.String("path", dbPath))

	// Create health checker
	healthChecker := observability.NewHealthChecker(logger)

	// Create Kafka producer for outbox publisher
	brokers := strings.Split(cfg.KafkaBrokers, ",")
	for i := range brokers {
		brokers[i] = strings.TrimSpace(brokers[i])
	}
	producer, err := msg.NewProducer(brokers, logger)
	if err != nil {
		logger.Fatal("failed to create kafka producer", zap.Error(err))
	}
	defer producer.Close()

	// Create outbox publisher
	publisher := idempotency.NewPublisher(store, producer, logger)

	// Create Kafka consumer for order commands
	consumer, err := msg.NewConsumer(brokers, "order-executor-v1", []string{msg.TopicOrdersCommands}, logger)
	if err != nil {
		logger.Fatal("failed to create kafka consumer", zap.Error(err))
	}
	defer consumer.Close()

	// Create gRPC server
	grpcServer := grpc.NewServer()
	healthChecker.RegisterGRPC(grpcServer)

	// Register OrderExecutorService
	orderExecutorServer := orderexecutor.NewServer(logger)
	tradingv1.RegisterOrderExecutorServiceServer(grpcServer, orderExecutorServer)

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

	// Start consumer
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	consumerErrCh := make(chan error, 1)
	go func() {
		err := consumer.Run(ctx, func(ctx context.Context, rec msg.Record) error {
			// Parse order command message
			var orderCmd msg.OrderCmdMsg
			if err := json.Unmarshal(rec.Value, &orderCmd); err != nil {
				return fmt.Errorf("failed to unmarshal order command: %w", err)
			}

			// Validate order
			if orderCmd.OrderID == "" {
				return fmt.Errorf("order_id cannot be empty")
			}
			if orderCmd.Qty <= 0 {
				return fmt.Errorf("qty must be greater than 0")
			}
			if orderCmd.Price <= 0 {
				return fmt.Errorf("price must be greater than 0")
			}
			if orderCmd.Symbol == "" {
				return fmt.Errorf("symbol cannot be empty")
			}

			// Process order command atomically (idempotent)
			result, err := store.ProcessOrderCommand(ctx, orderCmd)
			if err != nil {
				return fmt.Errorf("failed to process order command: %w", err)
			}

			if result.Duplicate {
				logger.Info("duplicate order command detected, skipping",
					zap.String("order_id", orderCmd.OrderID),
					zap.String("status", result.Status),
					zap.String("reason", result.Reason),
				)
				// Return nil to commit offset - order already processed
				return nil
			}

			logger.Info("order command processed",
				zap.String("order_id", orderCmd.OrderID),
				zap.String("event_id", orderCmd.EventID),
				zap.String("symbol", orderCmd.Symbol),
				zap.String("side", orderCmd.Side),
				zap.Int64("qty", orderCmd.Qty),
				zap.Float64("price", orderCmd.Price),
				zap.String("status", result.Status),
			)

			// Outbox event was created in the transaction
			// Publisher loop will handle publishing asynchronously
			// Return nil to commit offset after DB transaction succeeded
			return nil
		})
		if err != nil {
			consumerErrCh <- err
		}
	}()

	// Start outbox publisher loop
	publisherErrCh := make(chan error, 1)
	go func() {
		if err := publisher.Run(ctx); err != nil {
			publisherErrCh <- err
		}
	}()

	// Wait for consumer to start
	time.Sleep(1 * time.Second)
	if consumer.IsRunning() {
		healthChecker.SetKafkaReady(true)
	} else {
		logger.Warn("consumer not running yet")
	}

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
	case err := <-consumerErrCh:
		logger.Error("consumer error", zap.Error(err))
	case err := <-publisherErrCh:
		logger.Error("publisher error", zap.Error(err))
	}

	// Graceful shutdown
	logger.Info("shutting down gracefully...")

	// Stop consumer and publisher
	cancel()
	producer.Close()
	consumer.Close()
	store.Close()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Shutdown health checker
	if err := healthChecker.Shutdown(shutdownCtx); err != nil {
		logger.Error("error shutting down health checker", zap.Error(err))
	}

	// Shutdown gRPC server
	grpcServer.GracefulStop()

	logger.Info("order-executor service stopped")
}

package main

import (
	"context"
	"encoding/json"
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
	"github.com/ismaiel54/fault-tolerant-trading-pipeline/internal/rpc/streamprocessor"
	tradingv1 "github.com/ismaiel54/fault-tolerant-trading-pipeline/gen/proto/trading/v1"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

func main() {
	// Load configuration
	cfg := config.LoadConfig("stream-processor")

	// Initialize logger
	logger, err := logging.NewLogger(cfg.ServiceName, cfg.LogLevel)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to initialize logger: %v\n", err)
		os.Exit(1)
	}
	defer logger.Sync()

	logger.Info("starting stream-processor service",
		zap.Int("grpc_port", cfg.GRPCPort),
		zap.Int("http_port", cfg.HTTPPort),
		zap.String("kafka_brokers", cfg.KafkaBrokers),
	)

	// Create health checker
	healthChecker := observability.NewHealthChecker(logger)

	// Create Kafka producer for orders
	brokers := strings.Split(cfg.KafkaBrokers, ",")
	for i := range brokers {
		brokers[i] = strings.TrimSpace(brokers[i])
	}
	producer, err := msg.NewProducer(brokers, logger)
	if err != nil {
		logger.Fatal("failed to create kafka producer", zap.Error(err))
	}
	defer producer.Close()

	// Create Kafka consumer for ticks
	consumer, err := msg.NewConsumer(brokers, "stream-processor-v1", []string{msg.TopicMarketTicks}, logger)
	if err != nil {
		logger.Fatal("failed to create kafka consumer", zap.Error(err))
	}
	defer consumer.Close()

	// Create gRPC server
	grpcServer := grpc.NewServer()
	healthChecker.RegisterGRPC(grpcServer)

	// Register StreamProcessorService
	streamProcessorServer := streamprocessor.NewServer(logger)
	tradingv1.RegisterStreamProcessorServiceServer(grpcServer, streamProcessorServer)

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
			// Parse tick message
			var tickMsg msg.TickMsg
			if err := json.Unmarshal(rec.Value, &tickMsg); err != nil {
				return fmt.Errorf("failed to unmarshal tick: %w", err)
			}

			logger.Info("processing tick",
				zap.String("event_id", tickMsg.EventID),
				zap.String("symbol", tickMsg.Symbol),
				zap.Float64("price", tickMsg.Price),
			)

			// Transform tick into order command (simple dummy strategy)
			orderID := uuid.New().String()
			side := "BUY"
			if int(tickMsg.Price*100)%2 == 0 {
				side = "SELL"
			}

			orderCmd := msg.OrderCmdMsg{
				EventID:      uuid.New().String(),
				OrderID:      orderID,
				Symbol:       tickMsg.Symbol,
				Side:         side,
				Qty:          10,
				Price:        tickMsg.Price,
				TsUnixMillis: time.Now().UnixMilli(),
			}

			// Produce order command
			if err := producer.ProduceJSON(ctx, msg.TopicOrdersCommands, orderCmd.OrderID, orderCmd); err != nil {
				return fmt.Errorf("failed to produce order command: %w", err)
			}

			logger.Info("order command produced",
				zap.String("order_id", orderCmd.OrderID),
				zap.String("symbol", orderCmd.Symbol),
				zap.String("side", orderCmd.Side),
				zap.Float64("price", orderCmd.Price),
			)

			return nil
		})
		if err != nil {
			consumerErrCh <- err
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
	}

	// Graceful shutdown
	logger.Info("shutting down gracefully...")

	// Stop consumer
	cancel()
	producer.Close()
	consumer.Close()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Shutdown health checker
	if err := healthChecker.Shutdown(shutdownCtx); err != nil {
		logger.Error("error shutting down health checker", zap.Error(err))
	}

	// Shutdown gRPC server
	grpcServer.GracefulStop()

	logger.Info("stream-processor service stopped")
}

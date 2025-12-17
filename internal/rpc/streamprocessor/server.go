package streamprocessor

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/ismaiel54/fault-tolerant-trading-pipeline/gen/proto/trading/v1"
	"go.uber.org/zap"
)

// Server implements StreamProcessorService
type Server struct {
	tradingv1.UnimplementedStreamProcessorServiceServer
	logger *zap.Logger
}

// NewServer creates a new StreamProcessorService server
func NewServer(logger *zap.Logger) tradingv1.StreamProcessorServiceServer {
	return &Server{
		logger: logger,
	}
}

// ProcessTick processes a market data tick
func (s *Server) ProcessTick(ctx context.Context, tick *tradingv1.Tick) (*tradingv1.Ack, error) {
	// Validate tick
	if tick.Symbol == "" {
		return nil, fmt.Errorf("symbol cannot be empty")
	}
	if tick.Price <= 0 {
		return nil, fmt.Errorf("price must be greater than 0")
	}
	if tick.TsUnixMillis <= 0 {
		return nil, fmt.Errorf("ts_unix_millis must be greater than 0")
	}

	// Log tick fields
	s.logger.Info("processing tick",
		zap.String("symbol", tick.Symbol),
		zap.Float64("price", tick.Price),
		zap.Int64("ts_unix_millis", tick.TsUnixMillis),
	)

	// Generate request ID
	requestID := uuid.New().String()

	// Return ack
	return &tradingv1.Ack{
		RequestId: requestID,
		Message:   "tick accepted",
	}, nil
}


package orderexecutor

import (
	"context"
	"fmt"
	"time"

	"github.com/ismaiel54/fault-tolerant-trading-pipeline/gen/proto/trading/v1"
	"go.uber.org/zap"
)

// Server implements OrderExecutorService
type Server struct {
	tradingv1.UnimplementedOrderExecutorServiceServer
	logger *zap.Logger
}

// NewServer creates a new OrderExecutorService server
func NewServer(logger *zap.Logger) tradingv1.OrderExecutorServiceServer {
	return &Server{
		logger: logger,
	}
}

// SubmitOrder submits a trading order
func (s *Server) SubmitOrder(ctx context.Context, order *tradingv1.Order) (*tradingv1.OrderAck, error) {
	// Validate order
	if order.OrderId == "" {
		return nil, fmt.Errorf("order_id cannot be empty")
	}
	if order.Qty <= 0 {
		return nil, fmt.Errorf("qty must be greater than 0")
	}
	if order.Price <= 0 {
		return nil, fmt.Errorf("price must be greater than 0")
	}
	if order.Symbol == "" {
		return nil, fmt.Errorf("symbol cannot be empty")
	}

	// Log order
	s.logger.Info("submitting order",
		zap.String("order_id", order.OrderId),
		zap.String("symbol", order.Symbol),
		zap.String("side", order.Side.String()),
		zap.Int64("qty", order.Qty),
		zap.Float64("price", order.Price),
		zap.Int64("ts_unix_millis", order.TsUnixMillis),
	)

	// For now, always return ACCEPTED
	now := time.Now().UnixMilli()
	return &tradingv1.OrderAck{
		OrderId:      order.OrderId,
		Status:       tradingv1.OrderStatus_ORDER_STATUS_ACCEPTED,
		Reason:       "accepted",
		TsUnixMillis: now,
	}, nil
}


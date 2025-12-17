package streamprocessor

import (
	"context"
	"fmt"
	"time"

	"github.com/ismaiel54/fault-tolerant-trading-pipeline/gen/proto/trading/v1"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
)

// Client is a gRPC client for StreamProcessorService
type Client struct {
	cc  *grpc.ClientConn
	svc tradingv1.StreamProcessorServiceClient
}

// Dial creates a new client connection to the stream processor service
func Dial(ctx context.Context, addr string, logger *zap.Logger) (*Client, error) {
	// Create unary interceptor for logging
	unaryInterceptor := func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		start := time.Now()
		err := invoker(ctx, method, req, reply, cc, opts...)
		duration := time.Since(start)
		
		code := status.Code(err)
		logger.Info("gRPC call",
			zap.String("method", method),
			zap.Duration("duration", duration),
			zap.String("status_code", code.String()),
		)
		return err
	}

	conn, err := grpc.DialContext(ctx, addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithUnaryInterceptor(unaryInterceptor),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to dial stream processor: %w", err)
	}

	return &Client{
		cc:  conn,
		svc: tradingv1.NewStreamProcessorServiceClient(conn),
	}, nil
}

// ProcessTick sends a tick to the stream processor
func (c *Client) ProcessTick(ctx context.Context, tick *tradingv1.Tick) (*tradingv1.Ack, error) {
	// Add timeout to context if not already set
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	return c.svc.ProcessTick(ctx, tick)
}

// Close closes the client connection
func (c *Client) Close() error {
	if c.cc != nil {
		return c.cc.Close()
	}
	return nil
}


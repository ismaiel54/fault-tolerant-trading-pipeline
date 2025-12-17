package raftnode

import (
	"context"
	"fmt"

	"github.com/ismaiel54/fault-tolerant-trading-pipeline/gen/proto/trading/v1"
	"go.uber.org/zap"
)

// Server implements RaftNodeService
type Server struct {
	tradingv1.UnimplementedRaftNodeServiceServer
	logger *zap.Logger
}

// NewServer creates a new RaftNodeService server
func NewServer(logger *zap.Logger) tradingv1.RaftNodeServiceServer {
	return &Server{
		logger: logger,
	}
}

// Apply applies a command to the Raft log
func (s *Server) Apply(ctx context.Context, cmd *tradingv1.Command) (*tradingv1.ApplyAck, error) {
	// Validate command
	if cmd.CommandId == "" {
		return nil, fmt.Errorf("command_id cannot be empty")
	}
	if cmd.Kind == "" {
		return nil, fmt.Errorf("kind cannot be empty")
	}

	// Log command
	s.logger.Info("applying command",
		zap.String("command_id", cmd.CommandId),
		zap.String("kind", cmd.Kind),
		zap.Int64("ts_unix_millis", cmd.TsUnixMillis),
	)

	// Return stub response
	return &tradingv1.ApplyAck{
		CommandId:   cmd.CommandId,
		Applied:     true,
		LeaderHint:  "",
		Message:     "applied (stub)",
	}, nil
}

// Read performs a read query
func (s *Server) Read(ctx context.Context, query *tradingv1.Query) (*tradingv1.ReadResp, error) {
	// Validate query
	if query.QueryId == "" {
		return nil, fmt.Errorf("query_id cannot be empty")
	}

	// Log query
	s.logger.Info("reading query",
		zap.String("query_id", query.QueryId),
		zap.String("kind", query.Kind),
	)

	// Return stub response
	return &tradingv1.ReadResp{
		QueryId: query.QueryId,
		Found:   false,
		Message: "not implemented",
	}, nil
}


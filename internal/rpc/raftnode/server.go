package raftnode

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/ismaiel54/fault-tolerant-trading-pipeline/gen/proto/trading/v1"
	"github.com/ismaiel54/fault-tolerant-trading-pipeline/internal/chaos"
	"github.com/ismaiel54/fault-tolerant-trading-pipeline/internal/raft"
	hashicorpraft "github.com/hashicorp/raft"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Server implements RaftNodeService
type Server struct {
	tradingv1.UnimplementedRaftNodeServiceServer
	raftNode *raft.Node
	chaos    *chaos.Chaos
	logger   *zap.Logger
	nodeID   string
}

// NewServer creates a new RaftNodeService server
func NewServer(raftNode *raft.Node, chaos *chaos.Chaos, nodeID string, logger *zap.Logger) tradingv1.RaftNodeServiceServer {
	return &Server{
		raftNode: raftNode,
		chaos:    chaos,
		logger:   logger,
		nodeID:   nodeID,
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

	// Inject chaos delay
	if s.chaos != nil {
		if err := s.chaos.MaybeDelay(ctx, s.nodeID, "Apply"); err != nil {
			return nil, err
		}

		// Inject chaos drop
		if s.chaos.MaybeDrop(s.nodeID, "Apply") {
			return nil, status.Error(codes.Unavailable, "chaos dropped")
		}
	}

	// Check if this node is the leader
	if !s.raftNode.IsLeader() {
		leader := s.raftNode.Leader()
		return &tradingv1.ApplyAck{
			CommandId:   cmd.CommandId,
			Applied:     false,
			LeaderHint:  leader,
			Message:     fmt.Sprintf("not leader, leader is %s", leader),
		}, nil
	}

	// Apply command to Raft
	result, err := s.raftNode.Apply(ctx, cmd.Payload, 2*time.Second)
	if err != nil {
		s.logger.Error("failed to apply command",
			zap.String("command_id", cmd.CommandId),
			zap.String("kind", cmd.Kind),
			zap.Error(err),
		)
		return &tradingv1.ApplyAck{
			CommandId:   cmd.CommandId,
			Applied:     false,
			LeaderHint:  "",
			Message:     fmt.Sprintf("apply failed: %v", err),
		}, nil
	}

	// Check result for duplicate
	message := "applied"
	if upsertResult, ok := result.(raft.UpsertResult); ok {
		if upsertResult.Duplicate {
			message = "duplicate"
		}
	}

	s.logger.Info("command applied",
		zap.String("command_id", cmd.CommandId),
		zap.String("kind", cmd.Kind),
		zap.String("message", message),
	)

	return &tradingv1.ApplyAck{
		CommandId:  cmd.CommandId,
		Applied:    true,
		LeaderHint: "",
		Message:    message,
	}, nil
}

// Read performs a read query
func (s *Server) Read(ctx context.Context, query *tradingv1.Query) (*tradingv1.ReadResp, error) {
	// Validate query
	if query.QueryId == "" {
		return nil, fmt.Errorf("query_id cannot be empty")
	}

	// Inject chaos delay
	if s.chaos != nil {
		if err := s.chaos.MaybeDelay(ctx, s.nodeID, "Read"); err != nil {
			return nil, err
		}

		// Inject chaos drop
		if s.chaos.MaybeDrop(s.nodeID, "Read") {
			return nil, status.Error(codes.Unavailable, "chaos dropped")
		}
	}

	// Handle GET_PROCESSED_ORDER query
	if query.Kind == "GET_PROCESSED_ORDER" {
		var payload struct {
			OrderID string `json:"order_id"`
		}
		if err := json.Unmarshal(query.Payload, &payload); err != nil {
			return &tradingv1.ReadResp{
				QueryId: query.QueryId,
				Found:   false,
				Message: fmt.Sprintf("failed to unmarshal payload: %v", err),
			}, nil
		}

		// Read from FSM
		fsm := s.raftNode.GetFSM()
		order, found := fsm.GetProcessedOrder(payload.OrderID)

		if !found {
			return &tradingv1.ReadResp{
				QueryId: query.QueryId,
				Found:   false,
				Message: "order not found",
			}, nil
		}

		orderJSON, err := json.Marshal(order)
		if err != nil {
			return &tradingv1.ReadResp{
				QueryId: query.QueryId,
				Found:   false,
				Message: fmt.Sprintf("failed to marshal order: %v", err),
			}, nil
		}

		return &tradingv1.ReadResp{
			QueryId: query.QueryId,
			Found:   true,
			Payload: orderJSON,
			Message: "found",
		}, nil
	}

	return &tradingv1.ReadResp{
		QueryId: query.QueryId,
		Found:   false,
		Message: fmt.Sprintf("unknown query kind: %s", query.Kind),
	}, nil
}

// Join handles a join request
func (s *Server) Join(ctx context.Context, req *tradingv1.JoinRequest) (*tradingv1.JoinResponse, error) {
	// Inject chaos delay
	if s.chaos != nil {
		if err := s.chaos.MaybeDelay(ctx, s.nodeID, "Join"); err != nil {
			return nil, err
		}

		// Inject chaos drop
		if s.chaos.MaybeDrop(s.nodeID, "Join") {
			return nil, status.Error(codes.Unavailable, "chaos dropped")
		}
	}

	if !s.raftNode.IsLeader() {
		leader := s.raftNode.Leader()
		return &tradingv1.JoinResponse{
			Ok:          false,
			LeaderHint:  leader,
			Message:     fmt.Sprintf("not leader, leader is %s", leader),
		}, nil
	}

	// Add voter to cluster
	if err := s.raftNode.AddVoter(
		hashicorpraft.ServerID(req.NodeId),
		hashicorpraft.ServerAddress(req.RaftAddress),
	); err != nil {
		s.logger.Error("failed to add voter",
			zap.String("node_id", req.NodeId),
			zap.String("raft_address", req.RaftAddress),
			zap.Error(err),
		)
		return &tradingv1.JoinResponse{
			Ok:         false,
			LeaderHint: "",
			Message:    fmt.Sprintf("failed to add voter: %v", err),
		}, nil
	}

	s.logger.Info("node joined cluster",
		zap.String("node_id", req.NodeId),
		zap.String("raft_address", req.RaftAddress),
	)

	return &tradingv1.JoinResponse{
		Ok:         true,
		LeaderHint: "",
		Message:    "joined successfully",
	}, nil
}

// Leader returns leader information
func (s *Server) Leader(ctx context.Context, req *tradingv1.LeaderRequest) (*tradingv1.LeaderResponse, error) {
	leader := s.raftNode.Leader()
	isLeader := s.raftNode.IsLeader()

	return &tradingv1.LeaderResponse{
		LeaderRaftAddress: leader,
		IsLeader:          isLeader,
	}, nil
}

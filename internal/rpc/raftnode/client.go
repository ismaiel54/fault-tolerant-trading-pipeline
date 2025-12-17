package raftnode

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/ismaiel54/fault-tolerant-trading-pipeline/gen/proto/trading/v1"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// Client is a gRPC client for RaftNodeService with leader following
type Client struct {
	cc     *grpc.ClientConn
	svc    tradingv1.RaftNodeServiceClient
	logger *zap.Logger
	addr   string
}

// Dial creates a new client connection
func Dial(ctx context.Context, addr string, logger *zap.Logger) (*Client, error) {
	conn, err := grpc.DialContext(ctx, addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to dial raft node: %w", err)
	}

	return &Client{
		cc:     conn,
		svc:    tradingv1.NewRaftNodeServiceClient(conn),
		logger: logger,
		addr:   addr,
	}, nil
}

// Apply applies a command with leader following
func (c *Client) Apply(ctx context.Context, cmd *tradingv1.Command, maxRetries int) (*tradingv1.ApplyAck, error) {
	currentAddr := c.addr

	for attempt := 0; attempt < maxRetries; attempt++ {
		// Dial current address if changed
		if currentAddr != c.addr {
			c.cc.Close()
			conn, err := grpc.DialContext(ctx, currentAddr,
				grpc.WithTransportCredentials(insecure.NewCredentials()),
			)
			if err != nil {
				return nil, fmt.Errorf("failed to dial raft node: %w", err)
			}
			c.cc = conn
			c.svc = tradingv1.NewRaftNodeServiceClient(conn)
			c.addr = currentAddr
		}

		// Apply command
		ack, err := c.svc.Apply(ctx, cmd)
		if err != nil {
			return nil, fmt.Errorf("failed to apply command: %w", err)
		}

		// If applied successfully, return
		if ack.Applied {
			return ack, nil
		}

		// If not leader and we have a leader hint, try to find gRPC address
		if ack.LeaderHint != "" && attempt < maxRetries-1 {
			c.logger.Info("following leader redirect",
				zap.String("current", currentAddr),
				zap.String("leader_hint", ack.LeaderHint),
			)
			// Try common gRPC ports for the leader
			leaderGRPC := c.findLeaderGRPCAddr(ack.LeaderHint)
			if leaderGRPC != "" {
				currentAddr = leaderGRPC
				time.Sleep(100 * time.Millisecond)
				continue
			}
		}

		return ack, nil
	}

	return nil, fmt.Errorf("failed to apply command after %d retries", maxRetries)
}

// findLeaderGRPCAddr tries to find the gRPC address from Raft address
// This is a simplified mapping - in production you'd have proper service discovery
func (c *Client) findLeaderGRPCAddr(raftAddr string) string {
	// Extract host from Raft address
	parts := strings.Split(raftAddr, ":")
	if len(parts) != 2 {
		return ""
	}
	host := parts[0]
	raftPort := parts[1]

	// Map Raft ports to gRPC ports (7000->50070, 7001->50071, 7002->50072)
	portMap := map[string]string{
		"7000": "50070",
		"7001": "50071",
		"7002": "50072",
	}

	if grpcPort, ok := portMap[raftPort]; ok {
		return fmt.Sprintf("%s:%s", host, grpcPort)
	}

	return ""
}

// Read performs a read query
func (c *Client) Read(ctx context.Context, query *tradingv1.Query) (*tradingv1.ReadResp, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	return c.svc.Read(ctx, query)
}

// Close closes the client connection
func (c *Client) Close() error {
	if c.cc != nil {
		return c.cc.Close()
	}
	return nil
}

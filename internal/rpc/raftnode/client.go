package raftnode

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/ismaiel54/fault-tolerant-trading-pipeline/gen/proto/trading/v1"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
)

// Client is a gRPC client for RaftNodeService with leader following and retry logic
type Client struct {
	cc          *grpc.ClientConn
	svc         tradingv1.RaftNodeServiceClient
	logger      *zap.Logger
	addr        string
	knownAddrs  []string
	currentIdx  int
}

// Dial creates a new client connection
func Dial(ctx context.Context, addr string, logger *zap.Logger) (*Client, error) {
	conn, err := grpc.DialContext(ctx, addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to dial raft node: %w", err)
	}

	// Parse known addresses from env if available
	knownAddrs := []string{addr}
	if addrsEnv := os.Getenv("RAFT_NODE_GRPC_ADDRS"); addrsEnv != "" {
		knownAddrs = strings.Split(addrsEnv, ",")
		for i := range knownAddrs {
			knownAddrs[i] = strings.TrimSpace(knownAddrs[i])
		}
	}

	return &Client{
		cc:         conn,
		svc:        tradingv1.NewRaftNodeServiceClient(conn),
		logger:     logger,
		addr:       addr,
		knownAddrs: knownAddrs,
		currentIdx: 0,
	}, nil
}

// Apply applies a command with leader following and exponential backoff retry
func (c *Client) Apply(ctx context.Context, cmd *tradingv1.Command, maxRetries int) (*tradingv1.ApplyAck, error) {
	currentAddr := c.addr
	backoffMs := 100

	for attempt := 0; attempt < maxRetries; attempt++ {
		// Dial current address if changed
		if currentAddr != c.addr {
			c.cc.Close()
			conn, err := grpc.DialContext(ctx, currentAddr,
				grpc.WithTransportCredentials(insecure.NewCredentials()),
			)
			if err != nil {
				c.logger.Warn("failed to dial new address",
					zap.String("addr", currentAddr),
					zap.Error(err),
				)
				// Try next address in round-robin
				currentAddr = c.nextAddr()
				time.Sleep(time.Duration(backoffMs) * time.Millisecond)
				backoffMs *= 2
				continue
			}
			c.cc = conn
			c.svc = tradingv1.NewRaftNodeServiceClient(conn)
			c.addr = currentAddr
		}

		// Create context with timeout for this attempt
		attemptCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
		defer cancel()

		// Apply command
		ack, err := c.svc.Apply(attemptCtx, cmd)
		if err != nil {
			// Check if it's a retryable error
			if isRetryableError(err) {
				c.logger.Info("retryable error, retrying",
					zap.Int("attempt", attempt+1),
					zap.Int("max_retries", maxRetries),
					zap.String("error", err.Error()),
					zap.Int("backoff_ms", backoffMs),
				)
				time.Sleep(time.Duration(backoffMs) * time.Millisecond)
				backoffMs *= 2
				continue
			}
			return nil, fmt.Errorf("failed to apply command: %w", err)
		}

		// If applied successfully, return
		if ack.Applied {
			c.logger.Debug("command applied successfully",
				zap.String("command_id", cmd.CommandId),
				zap.Int("attempt", attempt+1),
			)
			return ack, nil
		}

		// If not leader and we have a leader hint, try leader
		if ack.LeaderHint != "" && attempt < maxRetries-1 {
			c.logger.Info("following leader redirect",
				zap.String("current", currentAddr),
				zap.String("leader_hint", ack.LeaderHint),
				zap.Int("attempt", attempt+1),
			)
			leaderGRPC := c.findLeaderGRPCAddr(ack.LeaderHint)
			if leaderGRPC != "" {
				currentAddr = leaderGRPC
				time.Sleep(time.Duration(backoffMs) * time.Millisecond)
				backoffMs *= 2
				continue
			}
		}

		// If no leader hint, try round-robin
		if attempt < maxRetries-1 {
			currentAddr = c.nextAddr()
			time.Sleep(time.Duration(backoffMs) * time.Millisecond)
			backoffMs *= 2
			continue
		}

		return ack, nil
	}

	return nil, fmt.Errorf("failed to apply command after %d retries", maxRetries)
}

// Read performs a read query with retry
func (c *Client) Read(ctx context.Context, query *tradingv1.Query) (*tradingv1.ReadResp, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	return c.svc.Read(ctx, query)
}

// findLeaderGRPCAddr tries to find the gRPC address from Raft address
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

// nextAddr returns the next address in round-robin fashion
func (c *Client) nextAddr() string {
	if len(c.knownAddrs) == 0 {
		return c.addr
	}
	c.currentIdx = (c.currentIdx + 1) % len(c.knownAddrs)
	return c.knownAddrs[c.currentIdx]
}

// isRetryableError checks if an error is retryable
func isRetryableError(err error) bool {
	if err == nil {
		return false
	}

	st, ok := status.FromError(err)
	if !ok {
		return false
	}

	// Retry on Unavailable, DeadlineExceeded, and ResourceExhausted
	return st.Code() == codes.Unavailable ||
		st.Code() == codes.DeadlineExceeded ||
		st.Code() == codes.ResourceExhausted
}

// Close closes the client connection
func (c *Client) Close() error {
	if c.cc != nil {
		return c.cc.Close()
	}
	return nil
}


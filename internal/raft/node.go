package raft

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/hashicorp/raft"
	raftboltdb "github.com/hashicorp/raft-boltdb"
	"go.uber.org/zap"
)

// Node wraps a HashiCorp Raft node
type Node struct {
	raft          *raft.Raft
	fsm           *FSM
	config        *Config
	logger        *zap.Logger
	exitOnLeader  bool
	leaderCh      chan bool
	shutdownCh    chan struct{}
}

// Start starts a Raft node
func Start(ctx context.Context, cfg *Config, logger *zap.Logger) (*Node, error) {
	// Create data directory
	if err := os.MkdirAll(cfg.DataDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create data directory: %w", err)
	}

	// Create FSM
	fsm := NewFSM()

	// Create Raft configuration
	raftConfig := raft.DefaultConfig()
	raftConfig.LocalID = raft.ServerID(cfg.NodeID)
	raftConfig.SnapshotInterval = time.Duration(cfg.SnapshotInterval) * time.Second
	raftConfig.SnapshotThreshold = uint64(cfg.SnapshotThreshold)

	// Create log store
	logStorePath := filepath.Join(cfg.DataDir, "raft.db")
	logStore, err := raftboltdb.NewBoltStore(logStorePath)
	if err != nil {
		return nil, fmt.Errorf("failed to create log store: %w", err)
	}

	// Create stable store
	stableStorePath := filepath.Join(cfg.DataDir, "stable.db")
	stableStore, err := raftboltdb.NewBoltStore(stableStorePath)
	if err != nil {
		return nil, fmt.Errorf("failed to create stable store: %w", err)
	}

	// Create snapshot store
	snapshotStore, err := raft.NewFileSnapshotStore(cfg.DataDir, 3, os.Stderr)
	if err != nil {
		return nil, fmt.Errorf("failed to create snapshot store: %w", err)
	}

	// Create transport
	transport, err := raft.NewTCPTransport(cfg.BindAddr, nil, 3, 10*time.Second, os.Stderr)
	if err != nil {
		return nil, fmt.Errorf("failed to create transport: %w", err)
	}

	// Create Raft instance
	r, err := raft.NewRaft(raftConfig, fsm, logStore, stableStore, snapshotStore, transport)
	if err != nil {
		return nil, fmt.Errorf("failed to create raft: %w", err)
	}

	node := &Node{
		raft:         r,
		fsm:          fsm,
		config:       cfg,
		logger:       logger,
		leaderCh:     make(chan bool, 1),
		shutdownCh:   make(chan struct{}),
	}

	// Start leader monitoring goroutine
	go node.monitorLeadership()

	// Bootstrap if configured
	if cfg.Bootstrap {
		configuration := raft.Configuration{
			Servers: []raft.Server{
				{
					ID:      raft.ServerID(cfg.NodeID),
					Address: raft.ServerAddress(cfg.AdvertiseAddr),
				},
			},
		}
		future := r.BootstrapCluster(configuration)
		if err := future.Error(); err != nil {
			// Ignore error if cluster already bootstrapped
			if err.Error() != "cluster already bootstrapped" {
				return nil, fmt.Errorf("failed to bootstrap cluster: %w", err)
			}
		}
		logger.Info("bootstrapped Raft cluster", zap.String("node_id", cfg.NodeID))
	}

	logger.Info("Raft node started",
		zap.String("node_id", cfg.NodeID),
		zap.String("bind_addr", cfg.BindAddr),
		zap.String("advertise_addr", cfg.AdvertiseAddr),
		zap.Bool("bootstrap", cfg.Bootstrap),
	)

	return node, nil
}

// IsLeader returns whether this node is the leader
func (n *Node) IsLeader() bool {
	return n.raft.State() == raft.Leader
}

// Leader returns the current leader address
func (n *Node) Leader() string {
	return string(n.raft.Leader())
}

// Apply applies a command to the Raft log
func (n *Node) Apply(ctx context.Context, cmd []byte, timeout time.Duration) (interface{}, error) {
	if !n.IsLeader() {
		return nil, fmt.Errorf("not leader")
	}

	applyFuture := n.raft.Apply(cmd, timeout)
	if err := applyFuture.Error(); err != nil {
		return nil, fmt.Errorf("failed to apply command: %w", err)
	}

	return applyFuture.Response(), nil
}

// GetFSM returns the FSM (for reads)
func (n *Node) GetFSM() *FSM {
	return n.fsm
}

// AddVoter adds a voter to the cluster
func (n *Node) AddVoter(serverID raft.ServerID, address raft.ServerAddress) error {
	if !n.IsLeader() {
		return fmt.Errorf("not leader")
	}

	future := n.raft.AddVoter(serverID, address, 0, 0)
	return future.Error()
}

// SetExitOnLeader sets whether the node should exit when becoming leader
func (n *Node) SetExitOnLeader(exit bool) {
	n.exitOnLeader = exit
}

// monitorLeadership monitors leadership changes and exits if configured
func (n *Node) monitorLeadership() {
	wasLeader := false
	for {
		select {
		case <-n.shutdownCh:
			return
		case <-time.After(500 * time.Millisecond):
			isLeader := n.IsLeader()
			if isLeader && !wasLeader {
				// Just became leader
				n.logger.Info("became leader", zap.String("node_id", n.config.NodeID))
				if n.exitOnLeader {
					n.logger.Warn("exiting due to CHAOS_EXIT_ON_LEADER", zap.String("node_id", n.config.NodeID))
					// Give a short delay for graceful shutdown
					time.Sleep(2 * time.Second)
					os.Exit(0)
				}
			}
			wasLeader = isLeader
		}
	}
}

// Shutdown shuts down the Raft node
func (n *Node) Shutdown() error {
	close(n.shutdownCh)
	if n.raft != nil {
		future := n.raft.Shutdown()
		return future.Error()
	}
	return nil
}


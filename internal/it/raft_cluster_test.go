package it

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ismaiel54/fault-tolerant-trading-pipeline/internal/raft"
	"go.uber.org/zap"
)

func TestRaftCluster_LeaderElectionAndReplication(t *testing.T) {
	if os.Getenv("INTEGRATION") == "" {
		t.Skip("skipping integration test; set INTEGRATION=1 to run")
	}

	// Create temp directories
	baseDir := t.TempDir()
	node1Dir := filepath.Join(baseDir, "node1")
	node2Dir := filepath.Join(baseDir, "node2")
	node3Dir := filepath.Join(baseDir, "node3")

	os.MkdirAll(node1Dir, 0755)
	os.MkdirAll(node2Dir, 0755)
	os.MkdirAll(node3Dir, 0755)

	logger, _ := zap.NewDevelopment()

	// Start node 1 (bootstrap)
	cfg1 := &raft.Config{
		NodeID:            "node1",
		BindAddr:          "127.0.0.1:17000",
		AdvertiseAddr:     "127.0.0.1:17000",
		DataDir:           node1Dir,
		Bootstrap:         true,
		JoinAddr:          "",
		SnapshotInterval:  20,
		SnapshotThreshold: 64,
	}

	node1, err := raft.Start(context.Background(), cfg1, logger)
	if err != nil {
		t.Fatalf("failed to start node1: %v", err)
	}
	defer node1.Shutdown()

	// Wait for node1 to become leader
	time.Sleep(2 * time.Second)
	if !node1.IsLeader() {
		t.Fatal("node1 should be leader")
	}

	// Start node 2
	cfg2 := &raft.Config{
		NodeID:            "node2",
		BindAddr:          "127.0.0.1:17001",
		AdvertiseAddr:     "127.0.0.1:17001",
		DataDir:           node2Dir,
		Bootstrap:         false,
		JoinAddr:          "",
		SnapshotInterval:  20,
		SnapshotThreshold: 64,
	}

	node2, err := raft.Start(context.Background(), cfg2, logger)
	if err != nil {
		t.Fatalf("failed to start node2: %v", err)
	}
	defer node2.Shutdown()

	// Add node2 to cluster
	time.Sleep(1 * time.Second)
	if err := node1.AddVoter("node2", "127.0.0.1:17001"); err != nil {
		t.Fatalf("failed to add node2: %v", err)
	}

	// Start node 3
	cfg3 := &raft.Config{
		NodeID:            "node3",
		BindAddr:          "127.0.0.1:17002",
		AdvertiseAddr:     "127.0.0.1:17002",
		DataDir:           node3Dir,
		Bootstrap:         false,
		JoinAddr:          "",
		SnapshotInterval:  20,
		SnapshotThreshold: 64,
	}

	node3, err := raft.Start(context.Background(), cfg3, logger)
	if err != nil {
		t.Fatalf("failed to start node3: %v", err)
	}
	defer node3.Shutdown()

	// Add node3 to cluster
	time.Sleep(1 * time.Second)
	if err := node1.AddVoter("node3", "127.0.0.1:17002"); err != nil {
		t.Fatalf("failed to add node3: %v", err)
	}

	// Wait for cluster to stabilize
	time.Sleep(3 * time.Second)

	// Verify exactly one leader
	leaders := 0
	if node1.IsLeader() {
		leaders++
	}
	if node2.IsLeader() {
		leaders++
	}
	if node3.IsLeader() {
		leaders++
	}

	if leaders != 1 {
		t.Fatalf("expected exactly 1 leader, got %d", leaders)
	}

	// Apply a command on the leader
	var leaderNode *raft.Node
	if node1.IsLeader() {
		leaderNode = node1
	} else if node2.IsLeader() {
		leaderNode = node2
	} else {
		leaderNode = node3
	}

	cmd := raft.UpsertProcessedOrderCommand{
		OrderID:        "test-order-1",
		CommandEventID: "cmd-1",
		Status:         "ACCEPTED",
		Reason:         "test",
		TsUnixMillis:   time.Now().UnixMilli(),
	}

	cmdBytes, err := raft.EncodeCommand(raft.CommandKindUpsertProcessedOrder, cmd)
	if err != nil {
		t.Fatalf("failed to encode command: %v", err)
	}

	result, err := leaderNode.Apply(context.Background(), cmdBytes, 5*time.Second)
	if err != nil {
		t.Fatalf("failed to apply command: %v", err)
	}

	upsertResult, ok := result.(raft.UpsertResult)
	if !ok {
		t.Fatalf("unexpected result type: %T", result)
	}

	if upsertResult.Duplicate {
		t.Fatal("command should not be duplicate")
	}

	// Wait for replication
	time.Sleep(2 * time.Second)

	// Verify all nodes have the state
	fsm1 := node1.GetFSM()
	fsm2 := node2.GetFSM()
	fsm3 := node3.GetFSM()

	order1, found1 := fsm1.GetProcessedOrder("test-order-1")
	order2, found2 := fsm2.GetProcessedOrder("test-order-1")
	order3, found3 := fsm3.GetProcessedOrder("test-order-1")

	if !found1 || !found2 || !found3 {
		t.Fatalf("order not found on all nodes: node1=%v, node2=%v, node3=%v", found1, found2, found3)
	}

	if order1.OrderID != "test-order-1" || order2.OrderID != "test-order-1" || order3.OrderID != "test-order-1" {
		t.Fatal("order IDs don't match")
	}
}

func TestRaftCluster_Deduplication(t *testing.T) {
	if os.Getenv("INTEGRATION") == "" {
		t.Skip("skipping integration test; set INTEGRATION=1 to run")
	}

	baseDir := t.TempDir()
	nodeDir := filepath.Join(baseDir, "node1")
	os.MkdirAll(nodeDir, 0755)

	logger, _ := zap.NewDevelopment()

	cfg := &raft.Config{
		NodeID:            "node1",
		BindAddr:          "127.0.0.1:18000",
		AdvertiseAddr:     "127.0.0.1:18000",
		DataDir:           nodeDir,
		Bootstrap:         true,
		JoinAddr:          "",
		SnapshotInterval:  20,
		SnapshotThreshold: 64,
	}

	node, err := raft.Start(context.Background(), cfg, logger)
	if err != nil {
		t.Fatalf("failed to start node: %v", err)
	}
	defer node.Shutdown()

	time.Sleep(1 * time.Second)

	// Apply first command
	cmd1 := raft.UpsertProcessedOrderCommand{
		OrderID:        "dup-order-1",
		CommandEventID: "cmd-1",
		Status:         "ACCEPTED",
		Reason:         "first",
		TsUnixMillis:   time.Now().UnixMilli(),
	}

	cmdBytes1, _ := raft.EncodeCommand(raft.CommandKindUpsertProcessedOrder, cmd1)
	result1, err := node.Apply(context.Background(), cmdBytes1, 5*time.Second)
	if err != nil {
		t.Fatalf("failed to apply first command: %v", err)
	}

	upsertResult1, ok := result1.(raft.UpsertResult)
	if !ok || upsertResult1.Duplicate {
		t.Fatal("first command should not be duplicate")
	}

	// Apply duplicate command (same order_id, different command_event_id)
	cmd2 := raft.UpsertProcessedOrderCommand{
		OrderID:        "dup-order-1",
		CommandEventID: "cmd-2",
		Status:         "ACCEPTED",
		Reason:         "duplicate",
		TsUnixMillis:   time.Now().UnixMilli(),
	}

	cmdBytes2, _ := raft.EncodeCommand(raft.CommandKindUpsertProcessedOrder, cmd2)
	result2, err := node.Apply(context.Background(), cmdBytes2, 5*time.Second)
	if err != nil {
		t.Fatalf("failed to apply second command: %v", err)
	}

	upsertResult2, ok := result2.(raft.UpsertResult)
	if !ok || !upsertResult2.Duplicate {
		t.Fatal("second command should be duplicate")
	}

	// Verify FSM still has first record
	fsm := node.GetFSM()
	order, found := fsm.GetProcessedOrder("dup-order-1")
	if !found {
		t.Fatal("order should exist")
	}

	if order.CommandEventID != "cmd-1" {
		t.Fatalf("expected command_event_id 'cmd-1', got '%s'", order.CommandEventID)
	}
}


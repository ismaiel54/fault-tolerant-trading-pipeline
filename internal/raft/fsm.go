package raft

import (
	"encoding/json"
	"fmt"
	"io"
	"sync"

	"github.com/hashicorp/raft"
)

// FSM implements the Raft finite state machine
type FSM struct {
	mu        sync.RWMutex
	processed map[string]ProcessedOrder
}

// NewFSM creates a new FSM
func NewFSM() *FSM {
	return &FSM{
		processed: make(map[string]ProcessedOrder),
	}
}

// Apply applies a log entry to the FSM
func (f *FSM) Apply(log *raft.Log) interface{} {
	f.mu.Lock()
	defer f.mu.Unlock()

	// Decode command
	cmd, err := DecodeCommand(log.Data)
	if err != nil {
		return UpsertResult{Duplicate: false, OrderID: ""}
	}

	switch cmd.Kind {
	case CommandKindUpsertProcessedOrder:
		var upsertCmd UpsertProcessedOrderCommand
		if err := json.Unmarshal(cmd.Payload, &upsertCmd); err != nil {
			return UpsertResult{Duplicate: false, OrderID: upsertCmd.OrderID}
		}

		// Check if order already exists
		if _, exists := f.processed[upsertCmd.OrderID]; exists {
			// Duplicate - keep existing record
			return UpsertResult{
				Duplicate: true,
				OrderID:   upsertCmd.OrderID,
			}
		}

		// New order - insert it
		f.processed[upsertCmd.OrderID] = ProcessedOrder{
			OrderID:        upsertCmd.OrderID,
			CommandEventID: upsertCmd.CommandEventID,
			Status:         upsertCmd.Status,
			Reason:         upsertCmd.Reason,
			FirstSeenTs:    upsertCmd.TsUnixMillis,
		}

		return UpsertResult{
			Duplicate: false,
			OrderID:   upsertCmd.OrderID,
		}

	default:
		return UpsertResult{Duplicate: false, OrderID: ""}
	}
}

// Snapshot returns a snapshot of the FSM state
func (f *FSM) Snapshot() (raft.FSMSnapshot, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	// Create a copy of the state
	snapshot := make(map[string]ProcessedOrder)
	for k, v := range f.processed {
		snapshot[k] = v
	}

	return &fsmSnapshot{state: snapshot}, nil
}

// Restore restores the FSM from a snapshot
func (f *FSM) Restore(rc io.ReadCloser) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	defer rc.Close()

	// Decode snapshot
	var snapshot map[string]ProcessedOrder
	decoder := json.NewDecoder(rc)
	if err := decoder.Decode(&snapshot); err != nil {
		return fmt.Errorf("failed to decode snapshot: %w", err)
	}

	f.processed = snapshot
	return nil
}

// GetProcessedOrder returns a processed order by order_id
func (f *FSM) GetProcessedOrder(orderID string) (ProcessedOrder, bool) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	order, exists := f.processed[orderID]
	return order, exists
}

// GetStateSnapshot returns a thread-safe snapshot of the state
func (f *FSM) GetStateSnapshot() map[string]ProcessedOrder {
	f.mu.RLock()
	defer f.mu.RUnlock()

	snapshot := make(map[string]ProcessedOrder)
	for k, v := range f.processed {
		snapshot[k] = v
	}
	return snapshot
}

// fsmSnapshot implements raft.FSMSnapshot
type fsmSnapshot struct {
	state map[string]ProcessedOrder
}

func (s *fsmSnapshot) Persist(sink raft.SnapshotSink) error {
	encoder := json.NewEncoder(sink)
	if err := encoder.Encode(s.state); err != nil {
		sink.Cancel()
		return fmt.Errorf("failed to encode snapshot: %w", err)
	}
	return sink.Close()
}

func (s *fsmSnapshot) Release() {
	// No cleanup needed
}


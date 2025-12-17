package raft

import (
	"encoding/json"
	"fmt"
)

// CommandEnvelope wraps commands for Raft
type CommandEnvelope struct {
	Kind    string          `json:"kind"`
	Payload json.RawMessage `json:"payload"`
}

// Command kinds
const (
	CommandKindUpsertProcessedOrder = "UPSERT_PROCESSED_ORDER"
)

// UpsertProcessedOrderCommand represents a command to upsert a processed order
type UpsertProcessedOrderCommand struct {
	OrderID        string `json:"order_id"`
	CommandEventID string `json:"command_event_id"`
	Status         string `json:"status"`
	Reason         string `json:"reason"`
	TsUnixMillis   int64  `json:"ts_unix_millis"`
}

// ProcessedOrder represents a processed order in the FSM state
type ProcessedOrder struct {
	OrderID        string `json:"order_id"`
	CommandEventID string `json:"command_event_id"`
	Status         string `json:"status"`
	Reason         string `json:"reason"`
	FirstSeenTs    int64  `json:"first_seen_ts"`
}

// UpsertResult represents the result of an upsert operation
type UpsertResult struct {
	Duplicate bool   `json:"duplicate"`
	OrderID   string `json:"order_id"`
}

// EncodeCommand encodes a command into JSON bytes
func EncodeCommand(kind string, payload interface{}) ([]byte, error) {
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal payload: %w", err)
	}

	cmd := CommandEnvelope{
		Kind:    kind,
		Payload: payloadJSON,
	}

	return json.Marshal(cmd)
}

// DecodeCommand decodes a command from JSON bytes
func DecodeCommand(data []byte) (*CommandEnvelope, error) {
	var cmd CommandEnvelope
	if err := json.Unmarshal(data, &cmd); err != nil {
		return nil, fmt.Errorf("failed to unmarshal command: %w", err)
	}
	return &cmd, nil
}


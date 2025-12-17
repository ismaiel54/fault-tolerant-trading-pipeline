package idempotency

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/ismaiel54/fault-tolerant-trading-pipeline/internal/msg"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProcessOrderCommand_Idempotency(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "idempotency_test_*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	dbPath := filepath.Join(tmpDir, "test.db")
	store, err := Open(dbPath)
	require.NoError(t, err)
	defer store.Close()

	ctx := context.Background()

	// First command
	cmd1 := msg.OrderCmdMsg{
		EventID:      "evt-123",
		OrderID:      "ord-123",
		Symbol:       "AAPL",
		Side:         "BUY",
		Qty:          10,
		Price:        150.0,
		TsUnixMillis: 1000,
	}

	result1, err := store.ProcessOrderCommand(ctx, cmd1)
	require.NoError(t, err)
	assert.False(t, result1.Duplicate, "first call should not be duplicate")
	assert.Equal(t, "ACCEPTED", result1.Status)
	assert.NotNil(t, result1.OutboxEvent, "should create outbox event")
	assert.Equal(t, "evt-evt-123", result1.OutboxEvent.EventID, "event_id should be derived from command event_id")

	// Second command with same order_id (duplicate)
	cmd2 := msg.OrderCmdMsg{
		EventID:      "evt-456", // Different event_id but same order_id
		OrderID:      "ord-123", // Same order_id
		Symbol:       "AAPL",
		Side:         "BUY",
		Qty:          10,
		Price:        150.0,
		TsUnixMillis: 2000,
	}

	result2, err := store.ProcessOrderCommand(ctx, cmd2)
	require.NoError(t, err)
	assert.True(t, result2.Duplicate, "second call with same order_id should be duplicate")
	assert.Nil(t, result2.OutboxEvent, "should not create new outbox event for duplicate")

	// Verify only one outbox event exists
	unpublished, err := store.ListUnpublished(ctx, 100)
	require.NoError(t, err)
	assert.Len(t, unpublished, 1, "should have exactly one unpublished event")
	assert.Equal(t, "evt-evt-123", unpublished[0].EventID)
	assert.Equal(t, "ord-123", unpublished[0].OrderID)
}

func TestOutboxPublisher(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "outbox_test_*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	dbPath := filepath.Join(tmpDir, "test.db")
	store, err := Open(dbPath)
	require.NoError(t, err)
	defer store.Close()

	ctx := context.Background()

	// Create an order command and process it
	cmd := msg.OrderCmdMsg{
		EventID:      "evt-test",
		OrderID:      "ord-test",
		Symbol:       "AAPL",
		Side:         "BUY",
		Qty:          10,
		Price:        150.0,
		TsUnixMillis: 1000,
	}

	result, err := store.ProcessOrderCommand(ctx, cmd)
	require.NoError(t, err)
	require.NotNil(t, result.OutboxEvent)

	// List unpublished events
	unpublished, err := store.ListUnpublished(ctx, 100)
	require.NoError(t, err)
	assert.Len(t, unpublished, 1)

	// Mark as published
	err = store.MarkPublished(ctx, result.OutboxEvent.EventID, 2000)
	require.NoError(t, err)

	// List unpublished again - should be empty
	unpublished, err = store.ListUnpublished(ctx, 100)
	require.NoError(t, err)
	assert.Len(t, unpublished, 0, "should have no unpublished events after marking as published")
}


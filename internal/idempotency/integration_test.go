//go:build integration
// +build integration

package idempotency

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ismaiel54/fault-tolerant-trading-pipeline/internal/msg"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIntegration_DuplicateOrderCommand(t *testing.T) {
	if os.Getenv("INTEGRATION") != "1" {
		t.Skip("Skipping integration test. Set INTEGRATION=1 to run.")
	}

	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "integration_test_*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	dbPath := filepath.Join(tmpDir, "test.db")
	store, err := Open(dbPath)
	require.NoError(t, err)
	defer store.Close()

	ctx := context.Background()

	// Process same order command twice
	cmd := msg.OrderCmdMsg{
		EventID:      "evt-integration-test",
		OrderID:      "ord-integration-test",
		Symbol:       "AAPL",
		Side:         "BUY",
		Qty:          10,
		Price:        150.0,
		TsUnixMillis: time.Now().UnixMilli(),
	}

	// First processing
	result1, err := store.ProcessOrderCommand(ctx, cmd)
	require.NoError(t, err)
	assert.False(t, result1.Duplicate)
	assert.NotNil(t, result1.OutboxEvent)

	// Second processing (duplicate)
	result2, err := store.ProcessOrderCommand(ctx, cmd)
	require.NoError(t, err)
	assert.True(t, result2.Duplicate)
	assert.Nil(t, result2.OutboxEvent)

	// Verify outbox has exactly one event
	unpublished, err := store.ListUnpublished(ctx, 100)
	require.NoError(t, err)
	assert.Len(t, unpublished, 1)

	// Verify event content
	event := unpublished[0]
	var orderEvent msg.OrderEventMsg
	err = json.Unmarshal([]byte(event.PayloadJSON), &orderEvent)
	require.NoError(t, err)
	assert.Equal(t, "ord-integration-test", orderEvent.OrderID)
	assert.Equal(t, "ACCEPTED", orderEvent.Status)
}


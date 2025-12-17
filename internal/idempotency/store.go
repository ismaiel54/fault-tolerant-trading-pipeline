package idempotency

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
	"github.com/ismaiel54/fault-tolerant-trading-pipeline/internal/msg"
)

// Store provides idempotency and outbox functionality
type Store struct {
	db *sql.DB
}

// ProcessResult represents the result of processing an order command
type ProcessResult struct {
	Duplicate   bool
	Status      string
	Reason      string
	OutboxEvent *OutboxEvent
}

// OutboxEvent represents an event waiting to be published
type OutboxEvent struct {
	ID                int64
	OrderID           string
	EventID           string
	Topic             string
	Key               string
	PayloadJSON       string
	CreatedUnixMillis int64
	PublishedUnixMillis sql.NullInt64
}

// Open creates or opens the idempotency store
func Open(path string) (*Store, error) {
	// Create directory if it doesn't exist
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create data directory: %w", err)
	}

	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	store := &Store{db: db}

	// Run migrations
	if err := store.migrate(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to run migrations: %w", err)
	}

	return store, nil
}

// migrate creates the necessary tables
func (s *Store) migrate() error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS processed_commands (
			order_id TEXT PRIMARY KEY,
			command_event_id TEXT NOT NULL,
			first_seen_unix_millis INTEGER NOT NULL,
			status TEXT NOT NULL,
			reason TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS outbox_events (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			order_id TEXT NOT NULL,
			event_id TEXT NOT NULL UNIQUE,
			topic TEXT NOT NULL,
			key TEXT NOT NULL,
			payload_json TEXT NOT NULL,
			created_unix_millis INTEGER NOT NULL,
			published_unix_millis INTEGER NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_outbox_unpublished 
			ON outbox_events(published_unix_millis) 
			WHERE published_unix_millis IS NULL`,
	}

	for _, query := range queries {
		if _, err := s.db.Exec(query); err != nil {
			return fmt.Errorf("failed to execute migration: %w", err)
		}
	}

	return nil
}

// ProcessOrderCommand processes an order command atomically
func (s *Store) ProcessOrderCommand(ctx context.Context, cmd msg.OrderCmdMsg) (ProcessResult, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return ProcessResult{}, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Check if order_id already exists
	var existingCmdEventID string
	var existingStatus string
	err = tx.QueryRowContext(ctx,
		"SELECT command_event_id, status FROM processed_commands WHERE order_id = ?",
		cmd.OrderID,
	).Scan(&existingCmdEventID, &existingStatus)

	if err == nil {
		// Order already processed - duplicate
		// Check if outbox event exists
		var outboxEventID sql.NullString
		var outboxPayload sql.NullString
		err = tx.QueryRowContext(ctx,
			"SELECT event_id, payload_json FROM outbox_events WHERE order_id = ?",
			cmd.OrderID,
		).Scan(&outboxEventID, &outboxPayload)

		if err == nil && outboxEventID.Valid {
			// Outbox event exists
			return ProcessResult{
				Duplicate: true,
				Status:    existingStatus,
				Reason:    "order already processed",
			}, nil
		}

		// Order processed but no outbox event - should not happen, but handle gracefully
		return ProcessResult{
			Duplicate: true,
			Status:    existingStatus,
			Reason:    "order already processed (no outbox event found)",
		}, nil
	} else if err != sql.ErrNoRows {
		return ProcessResult{}, fmt.Errorf("failed to check existing order: %w", err)
	}

	// New order - process it
	now := time.Now().UnixMilli()

	// Insert into processed_commands
	_, err = tx.ExecContext(ctx,
		`INSERT INTO processed_commands (order_id, command_event_id, first_seen_unix_millis, status, reason)
		 VALUES (?, ?, ?, ?, ?)`,
		cmd.OrderID, cmd.EventID, now, "ACCEPTED", "accepted",
	)
	if err != nil {
		return ProcessResult{}, fmt.Errorf("failed to insert processed command: %w", err)
	}

	// Create order event
	eventID := "evt-" + cmd.EventID
	orderEvent := msg.OrderEventMsg{
		EventID:      eventID,
		OrderID:      cmd.OrderID,
		Status:       "ACCEPTED",
		Reason:       "accepted",
		TsUnixMillis: now,
	}

	eventJSON, err := json.Marshal(orderEvent)
	if err != nil {
		return ProcessResult{}, fmt.Errorf("failed to marshal order event: %w", err)
	}

	// Insert into outbox_events
	_, err = tx.ExecContext(ctx,
		`INSERT INTO outbox_events (order_id, event_id, topic, key, payload_json, created_unix_millis, published_unix_millis)
		 VALUES (?, ?, ?, ?, ?, ?, NULL)`,
		cmd.OrderID, eventID, "orders.events", cmd.OrderID, string(eventJSON), now,
	)
	if err != nil {
		return ProcessResult{}, fmt.Errorf("failed to insert outbox event: %w", err)
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		return ProcessResult{}, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return ProcessResult{
		Duplicate: false,
		Status:    "ACCEPTED",
		Reason:    "accepted",
		OutboxEvent: &OutboxEvent{
			OrderID:           cmd.OrderID,
			EventID:           eventID,
			Topic:             "orders.events",
			Key:               cmd.OrderID,
			PayloadJSON:       string(eventJSON),
			CreatedUnixMillis: now,
		},
	}, nil
}

// ListUnpublished returns unpublished outbox events
func (s *Store) ListUnpublished(ctx context.Context, limit int) ([]OutboxEvent, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, order_id, event_id, topic, key, payload_json, created_unix_millis, published_unix_millis
		 FROM outbox_events
		 WHERE published_unix_millis IS NULL
		 ORDER BY created_unix_millis ASC
		 LIMIT ?`,
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to query unpublished events: %w", err)
	}
	defer rows.Close()

	var events []OutboxEvent
	for rows.Next() {
		var e OutboxEvent
		var publishedUnixMillis sql.NullInt64
		err := rows.Scan(
			&e.ID, &e.OrderID, &e.EventID, &e.Topic, &e.Key,
			&e.PayloadJSON, &e.CreatedUnixMillis, &publishedUnixMillis,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan event: %w", err)
		}
		e.PublishedUnixMillis = publishedUnixMillis
		events = append(events, e)
	}

	return events, rows.Err()
}

// MarkPublished marks an event as published
func (s *Store) MarkPublished(ctx context.Context, eventID string, nowMillis int64) error {
	_, err := s.db.ExecContext(ctx,
		"UPDATE outbox_events SET published_unix_millis = ? WHERE event_id = ?",
		nowMillis, eventID,
	)
	if err != nil {
		return fmt.Errorf("failed to mark event as published: %w", err)
	}
	return nil
}

// Close closes the database connection
func (s *Store) Close() error {
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}


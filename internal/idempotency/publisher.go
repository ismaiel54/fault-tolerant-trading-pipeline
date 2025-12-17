package idempotency

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/ismaiel54/fault-tolerant-trading-pipeline/internal/msg"
	"go.uber.org/zap"
)

// Publisher publishes outbox events to Kafka
type Publisher struct {
	store    *Store
	producer *msg.Producer
	logger   *zap.Logger
	interval time.Duration
	batchSize int
}

// NewPublisher creates a new outbox publisher
func NewPublisher(store *Store, producer *msg.Producer, logger *zap.Logger) *Publisher {
	return &Publisher{
		store:     store,
		producer:  producer,
		logger:    logger,
		interval:  250 * time.Millisecond,
		batchSize: 100,
	}
}

// Run starts the publisher loop
func (p *Publisher) Run(ctx context.Context) error {
	ticker := time.NewTicker(p.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if err := p.publishBatch(ctx); err != nil {
				p.logger.Error("failed to publish batch", zap.Error(err))
				// Continue - will retry on next tick
			}
		}
	}
}

// publishBatch publishes a batch of unpublished events
func (p *Publisher) publishBatch(ctx context.Context) error {
	events, err := p.store.ListUnpublished(ctx, p.batchSize)
	if err != nil {
		return fmt.Errorf("failed to list unpublished events: %w", err)
	}

	if len(events) == 0 {
		return nil
	}

	now := time.Now().UnixMilli()
	published := 0

	for _, event := range events {
		// Parse payload
		var orderEvent msg.OrderEventMsg
		if err := json.Unmarshal([]byte(event.PayloadJSON), &orderEvent); err != nil {
			p.logger.Error("failed to unmarshal event payload",
				zap.String("event_id", event.EventID),
				zap.Error(err),
			)
			continue
		}

		// Publish to Kafka
		if err := p.producer.ProduceJSON(ctx, event.Topic, event.Key, orderEvent); err != nil {
			p.logger.Error("failed to produce event",
				zap.String("event_id", event.EventID),
				zap.String("order_id", event.OrderID),
				zap.Error(err),
			)
			// Continue with next event - this one will be retried
			continue
		}

		// Mark as published
		if err := p.store.MarkPublished(ctx, event.EventID, now); err != nil {
			p.logger.Error("failed to mark event as published",
				zap.String("event_id", event.EventID),
				zap.Error(err),
			)
			// Continue - worst case we republish (idempotent)
			continue
		}

		published++
		p.logger.Debug("published outbox event",
			zap.String("event_id", event.EventID),
			zap.String("order_id", event.OrderID),
		)
	}

	if published > 0 {
		p.logger.Info("published outbox batch",
			zap.Int("published", published),
			zap.Int("total", len(events)),
		)
	}

	return nil
}


package msg

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/twmb/franz-go/pkg/kgo"
	"go.uber.org/zap"
)

// Consumer wraps a Kafka consumer
type Consumer struct {
	client  *kgo.Client
	logger  *zap.Logger
	topics  []string
	group   string
	running int32
	pollCount int64
	errorCount int64
}

// NewConsumer creates a new Kafka consumer
func NewConsumer(brokers []string, group string, topics []string, logger *zap.Logger) (*Consumer, error) {
	opts := []kgo.Opt{
		kgo.SeedBrokers(brokers...),
		kgo.ConsumerGroup(group),
		kgo.ConsumeTopics(topics...),
		kgo.DisableAutoCommit(), // Manual commit after handler success
	}

	client, err := kgo.NewClient(opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create kafka client: %w", err)
	}

	c := &Consumer{
		client: client,
		logger: logger,
		topics: topics,
		group:  group,
	}

	// Log consumer initialization
	logger.Info("consumer initialized",
		zap.Strings("brokers", brokers),
		zap.String("group", group),
		zap.Strings("topics", topics),
	)

	// Start periodic logging
	go c.logStats()

	return c, nil
}

// Run starts consuming messages and calls handler for each record
func (c *Consumer) Run(ctx context.Context, handler func(context.Context, Record) error) error {
	c.logger.Info("starting consumer",
		zap.String("group", c.group),
		zap.Strings("topics", c.topics),
	)

	atomic.StoreInt32(&c.running, 1)
	defer atomic.StoreInt32(&c.running, 0)

	for {
		select {
		case <-ctx.Done():
			c.logger.Info("consumer stopping", zap.String("group", c.group))
			return ctx.Err()
		default:
			// Poll for records with timeout
			fetches := c.client.PollFetches(ctx)
			if fetches.IsClientClosed() {
				return fmt.Errorf("kafka client closed")
			}

			iter := fetches.RecordIter()
			for !iter.Done() {
				record := iter.Next()

				// Convert to our Record type
				rec := Record{
					Topic:     record.Topic,
					Key:       string(record.Key),
					Value:     record.Value,
					Partition: record.Partition,
					Offset:    record.Offset,
					Timestamp: record.Timestamp.UnixMilli(),
				}

				// Call handler with retry logic
				err := c.handleWithRetry(ctx, rec, handler)
				if err != nil {
					c.logger.Error("handler failed after retries",
						zap.String("topic", rec.Topic),
						zap.String("key", rec.Key),
						zap.Error(err),
					)
					atomic.AddInt64(&c.errorCount, 1)
					// Continue processing other records
					continue
				}

				// Commit offset after successful handling
				c.client.CommitRecords(ctx, record)
				atomic.AddInt64(&c.pollCount, 1)
			}
		}
	}
}

// handleWithRetry calls handler with bounded retries
func (c *Consumer) handleWithRetry(ctx context.Context, rec Record, handler func(context.Context, Record) error) error {
	maxRetries := 3
	backoff := 100 * time.Millisecond

	for attempt := 0; attempt < maxRetries; attempt++ {
		err := handler(ctx, rec)
		if err == nil {
			return nil
		}

		if attempt < maxRetries-1 {
			c.logger.Warn("handler failed, retrying",
				zap.String("topic", rec.Topic),
				zap.String("key", rec.Key),
				zap.Int("attempt", attempt+1),
				zap.Error(err),
			)
			time.Sleep(backoff)
			backoff *= 2 // Exponential backoff
		}
	}

	return fmt.Errorf("handler failed after %d attempts", maxRetries)
}

// Close closes the consumer
func (c *Consumer) Close() {
	if c.client != nil {
		c.client.Close()
	}
}

// IsRunning returns whether the consumer is running
func (c *Consumer) IsRunning() bool {
	return atomic.LoadInt32(&c.running) == 1
}

// logStats logs consumer statistics periodically
func (c *Consumer) logStats() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		polls := atomic.LoadInt64(&c.pollCount)
		errors := atomic.LoadInt64(&c.errorCount)
		c.logger.Info("consumer stats",
			zap.String("group", c.group),
			zap.Int64("processed", polls),
			zap.Int64("errors", errors),
		)
	}
}


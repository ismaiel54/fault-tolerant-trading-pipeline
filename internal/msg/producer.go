package msg

import (
	"context"
	"encoding/json"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/twmb/franz-go/pkg/kgo"
	"go.uber.org/zap"
)

// Producer wraps a Kafka producer
type Producer struct {
	client *kgo.Client
	logger *zap.Logger
	produceCount int64
	errorCount   int64
}

// NewProducer creates a new Kafka producer
func NewProducer(brokers []string, logger *zap.Logger) (*Producer, error) {
	opts := []kgo.Opt{
		kgo.SeedBrokers(brokers...),
		kgo.RequiredAcks(kgo.AllISRAcks()),
		kgo.DisableIdempotentWrite(), // For simplicity, can enable later
	}

	client, err := kgo.NewClient(opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create kafka client: %w", err)
	}

	p := &Producer{
		client: client,
		logger: logger,
	}

	// Log broker connection
	logger.Info("producer initialized",
		zap.Strings("brokers", brokers),
	)

	// Start periodic logging
	go p.logStats()

	return p, nil
}

// ProduceJSON produces a JSON message to the specified topic
func (p *Producer) ProduceJSON(ctx context.Context, topic string, key string, v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		atomic.AddInt64(&p.errorCount, 1)
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	record := &kgo.Record{
		Topic: topic,
		Key:   []byte(key),
		Value: data,
	}

	// Synchronous produce with timeout
	produceCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	result := p.client.ProduceSync(produceCtx, record)
	if result.FirstErr() != nil {
		atomic.AddInt64(&p.errorCount, 1)
		return fmt.Errorf("failed to produce message: %w", result.FirstErr())
	}

	atomic.AddInt64(&p.produceCount, 1)
	return nil
}

// Close closes the producer
func (p *Producer) Close() {
	if p.client != nil {
		p.client.Close()
	}
}

// logStats logs producer statistics periodically
func (p *Producer) logStats() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		produced := atomic.LoadInt64(&p.produceCount)
		errors := atomic.LoadInt64(&p.errorCount)
		p.logger.Info("producer stats",
			zap.Int64("produced", produced),
			zap.Int64("errors", errors),
		)
	}
}


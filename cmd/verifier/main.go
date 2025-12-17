package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/ismaiel54/fault-tolerant-trading-pipeline/internal/logging"
	"github.com/ismaiel54/fault-tolerant-trading-pipeline/internal/msg"
	"go.uber.org/zap"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: %s <duration_seconds> [brokers]\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Example: %s 30 127.0.0.1:9092\n", os.Args[0])
		os.Exit(1)
	}

	var durationSeconds int
	if _, err := fmt.Sscanf(os.Args[1], "%d", &durationSeconds); err != nil {
		fmt.Fprintf(os.Stderr, "Invalid duration: %v\n", err)
		os.Exit(1)
	}

	brokers := "127.0.0.1:9092"
	if len(os.Args) >= 3 {
		brokers = os.Args[2]
	}

	logger, err := logging.NewLogger("verifier", "info")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create logger: %v\n", err)
		os.Exit(1)
	}
	defer logger.Sync()

	brokerList := strings.Split(brokers, ",")
	for i := range brokerList {
		brokerList[i] = strings.TrimSpace(brokerList[i])
	}

	logger.Info("starting verifier",
		zap.Int("duration_seconds", durationSeconds),
		zap.Strings("brokers", brokerList),
	)

	// Create consumer
	consumer, err := msg.NewConsumer(brokerList, "verifier-v1", []string{msg.TopicOrdersEvents}, logger)
	if err != nil {
		logger.Fatal("failed to create consumer", zap.Error(err))
	}
	defer consumer.Close()

	// Track order IDs and their counts
	orderCounts := make(map[string]int)
	eventIDs := make(map[string]string) // order_id -> first event_id

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(durationSeconds)*time.Second)
	defer cancel()

	// Consume events
	err = consumer.Run(ctx, func(ctx context.Context, rec msg.Record) error {
		var event msg.OrderEventMsg
		if err := json.Unmarshal(rec.Value, &event); err != nil {
			logger.Warn("failed to unmarshal event", zap.Error(err))
			return nil // Continue processing
		}

		orderCounts[event.OrderID]++
		if _, exists := eventIDs[event.OrderID]; !exists {
			eventIDs[event.OrderID] = event.EventID
		}

		logger.Debug("consumed event",
			zap.String("order_id", event.OrderID),
			zap.String("event_id", event.EventID),
			zap.String("status", event.Status),
			zap.String("topic", rec.Topic),
			zap.Int32("partition", rec.Partition),
			zap.Int64("offset", rec.Offset),
		)

		return nil
	})

	if err != nil && err != context.DeadlineExceeded {
		logger.Error("consumer error", zap.Error(err))
	}

	// Analyze results
	totalEvents := 0
	uniqueOrders := len(orderCounts)
	duplicates := make(map[string]int)

	for orderID, count := range orderCounts {
		totalEvents += count
		if count > 1 {
			duplicates[orderID] = count
		}
	}

	// Print results
	fmt.Println("\n=== Verification Results ===")
	fmt.Printf("Total events consumed: %d\n", totalEvents)
	fmt.Printf("Unique order IDs: %d\n", uniqueOrders)
	fmt.Printf("Duplicate order IDs: %d\n", len(duplicates))

	if len(duplicates) > 0 {
		fmt.Println("\nDuplicates found:")
		for orderID, count := range duplicates {
			fmt.Printf("  Order ID: %s, Count: %d, First Event ID: %s\n",
				orderID, count, eventIDs[orderID])
		}
		fmt.Println("\n❌ VERIFICATION FAILED: Duplicates detected!")
		os.Exit(1)
	}

	fmt.Println("\n✅ VERIFICATION PASSED: No duplicates detected!")
	os.Exit(0)
}


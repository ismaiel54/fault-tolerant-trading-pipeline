package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"math/rand"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/ismaiel54/fault-tolerant-trading-pipeline/internal/logging"
	"github.com/ismaiel54/fault-tolerant-trading-pipeline/internal/msg"
	"go.uber.org/zap"
)

func main() {
	var (
		count   = flag.Int("count", 50, "Number of orders to produce")
		dupPct  = flag.Int("dup-pct", 30, "Percentage of duplicates (0-100)")
		seed    = flag.Int64("seed", 42, "Random seed for deterministic generation")
		brokers = flag.String("brokers", "127.0.0.1:9092", "Kafka broker addresses")
		topic   = flag.String("topic", "orders.commands", "Topic to produce to (orders.commands or market.ticks)")
	)
	flag.Parse()

	logger, err := logging.NewLogger("producer", "info")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create logger: %v\n", err)
		os.Exit(1)
	}
	defer logger.Sync()

	brokerList := parseBrokers(*brokers)
	logger.Info("starting producer",
		zap.Int("count", *count),
		zap.Int("dup_pct", *dupPct),
		zap.Int64("seed", *seed),
		zap.Strings("brokers", brokerList),
		zap.String("topic", *topic),
	)

	// Create producer
	producer, err := msg.NewProducer(brokerList, logger)
	if err != nil {
		logger.Fatal("failed to create producer", zap.Error(err))
	}
	defer producer.Close()

	// Create deterministic RNG
	rng := rand.New(rand.NewSource(*seed))

	// Generate orders
	orders := make([]msg.OrderCmdMsg, 0, *count)
	orderSet := make(map[string]bool) // Track unique order IDs

	uniqueCount := 0
	dupCount := 0

	for i := 0; i < *count; i++ {
		// Decide if this should be a duplicate
		isDup := rng.Intn(100) < *dupPct && len(orderSet) > 0

		var orderID string
		if isDup {
			// Pick a random existing order ID
			keys := make([]string, 0, len(orderSet))
			for k := range orderSet {
				keys = append(keys, k)
			}
			orderID = keys[rng.Intn(len(keys))]
			dupCount++
		} else {
			// Generate new order ID
			orderID = fmt.Sprintf("ord-%d-%d", *seed, uniqueCount)
			orderSet[orderID] = true
			uniqueCount++
		}

		eventID := uuid.New().String()
		now := time.Now().UnixMilli()

		order := msg.OrderCmdMsg{
			EventID:      eventID,
			OrderID:     orderID,
			Symbol:      "AAPL",
			Side:        "BUY",
			Qty:         100,
			Price:       150.0 + rng.Float64()*10.0,
			TsUnixMillis: now,
		}

		orders = append(orders, order)
	}

	// Produce orders
	ctx := context.Background()
	produced := 0
	failed := 0

	for _, order := range orders {
		orderJSON, err := json.Marshal(order)
		if err != nil {
			logger.Error("failed to marshal order", zap.Error(err))
			failed++
			continue
		}

		if err := producer.ProduceJSON(ctx, *topic, order.OrderID, orderJSON); err != nil {
			logger.Error("failed to produce order",
				zap.String("order_id", order.OrderID),
				zap.Error(err),
			)
			failed++
			continue
		}

		produced++
		logger.Debug("produced order",
			zap.String("order_id", order.OrderID),
			zap.String("event_id", order.EventID),
		)
	}

	logger.Info("producer completed",
		zap.Int("total", *count),
		zap.Int("produced", produced),
		zap.Int("failed", failed),
		zap.Int("unique_orders", len(orderSet)),
		zap.Int("duplicates", dupCount),
	)

	fmt.Printf("\n=== Producer Summary ===\n")
	fmt.Printf("Total orders: %d\n", *count)
	fmt.Printf("Produced: %d\n", produced)
	fmt.Printf("Failed: %d\n", failed)
	fmt.Printf("Unique order IDs: %d\n", len(orderSet))
	fmt.Printf("Duplicate orders: %d\n", dupCount)
	fmt.Printf("Topic: %s\n", *topic)
	fmt.Printf("\n")

	if failed > 0 {
		os.Exit(1)
	}
}

func parseBrokers(brokers string) []string {
	brokerList := make([]string, 0)
	for _, b := range splitAndTrim(brokers, ",") {
		if b != "" {
			brokerList = append(brokerList, b)
		}
	}
	return brokerList
}

func splitAndTrim(s, sep string) []string {
	parts := make([]string, 0)
	for _, p := range splitString(s, sep) {
		trimmed := trimSpace(p)
		if trimmed != "" {
			parts = append(parts, trimmed)
		}
	}
	return parts
}

func splitString(s, sep string) []string {
	if sep == "" {
		return []string{s}
	}
	result := make([]string, 0)
	start := 0
	for i := 0; i < len(s); i++ {
		if i+len(sep) <= len(s) && s[i:i+len(sep)] == sep {
			result = append(result, s[start:i])
			start = i + len(sep)
			i += len(sep) - 1
		}
	}
	result = append(result, s[start:])
	return result
}

func trimSpace(s string) string {
	start := 0
	end := len(s)
	for start < end && (s[start] == ' ' || s[start] == '\t' || s[start] == '\n' || s[start] == '\r') {
		start++
	}
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t' || s[end-1] == '\n' || s[end-1] == '\r') {
		end--
	}
	return s[start:end]
}


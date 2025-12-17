package msg

import (
	"os"
	"strings"
)

// Config holds Kafka configuration
type Config struct {
	Brokers []string
	ClientID string
}

// Topic names
const (
	TopicMarketTicks    = "market.ticks"
	TopicOrdersCommands = "orders.commands"
	TopicOrdersEvents   = "orders.events"
)

// LoadConfig loads Kafka configuration from environment variables
func LoadConfig() *Config {
	brokersStr := getEnvAsString("KAFKA_BROKERS", "127.0.0.1:9092")
	brokers := strings.Split(brokersStr, ",")
	for i := range brokers {
		brokers[i] = strings.TrimSpace(brokers[i])
	}

	return &Config{
		Brokers:  brokers,
		ClientID: getEnvAsString("KAFKA_CLIENT_ID", "trading-pipeline"),
	}
}

func getEnvAsString(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}


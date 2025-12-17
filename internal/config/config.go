package config

import (
	"fmt"
	"os"
	"strconv"
)

// Config holds configuration for all services
type Config struct {
	// Service name
	ServiceName string

	// gRPC server port
	GRPCPort int

	// HTTP server port
	HTTPPort int

	// Log level: debug, info, warn, error
	LogLevel string
}

// LoadConfig loads configuration from environment variables with defaults
func LoadConfig(serviceName string) *Config {
	cfg := &Config{
		ServiceName: serviceName,
		GRPCPort:    getEnvAsInt("PORT_GRPC", 50051),
		HTTPPort:    getEnvAsInt("PORT_HTTP", 8080),
		LogLevel:    getEnvAsString("LOG_LEVEL", "info"),
	}

	return cfg
}

// GRPCAddr returns the gRPC server address
func (c *Config) GRPCAddr() string {
	return fmt.Sprintf(":%d", c.GRPCPort)
}

// HTTPAddr returns the HTTP server address
func (c *Config) HTTPAddr() string {
	return fmt.Sprintf(":%d", c.HTTPPort)
}

func getEnvAsString(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvAsInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

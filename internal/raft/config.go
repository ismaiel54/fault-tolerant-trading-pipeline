package raft

import (
	"fmt"
	"os"
	"strconv"
)

// Config holds Raft configuration
type Config struct {
	NodeID            string
	BindAddr          string
	AdvertiseAddr     string
	DataDir           string
	Bootstrap         bool
	JoinAddr          string
	SnapshotInterval  int
	SnapshotThreshold int
}

// LoadConfig loads Raft configuration from environment variables
func LoadConfig() (*Config, error) {
	nodeID := os.Getenv("RAFT_NODE_ID")
	if nodeID == "" {
		return nil, fmt.Errorf("RAFT_NODE_ID is required")
	}

	bindAddr := getEnvAsString("RAFT_BIND_ADDR", "127.0.0.1:7000")
	advertiseAddr := getEnvAsString("RAFT_ADVERTISE_ADDR", bindAddr)
	dataDir := getEnvAsString("RAFT_DATA_DIR", fmt.Sprintf("./.data/raft/%s", nodeID))
	bootstrap := getEnvAsBool("RAFT_BOOTSTRAP", false)
	joinAddr := getEnvAsString("RAFT_JOIN_ADDR", "")
	snapshotInterval := getEnvAsInt("RAFT_SNAPSHOT_INTERVAL", 20)
	snapshotThreshold := getEnvAsInt("RAFT_SNAPSHOT_THRESHOLD", 64)

	return &Config{
		NodeID:            nodeID,
		BindAddr:          bindAddr,
		AdvertiseAddr:     advertiseAddr,
		DataDir:           dataDir,
		Bootstrap:         bootstrap,
		JoinAddr:          joinAddr,
		SnapshotInterval:  snapshotInterval,
		SnapshotThreshold: snapshotThreshold,
	}, nil
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

func getEnvAsBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if boolValue, err := strconv.ParseBool(value); err == nil {
			return boolValue
		}
	}
	return defaultValue
}


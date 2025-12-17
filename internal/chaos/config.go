package chaos

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Config holds chaos configuration
type Config struct {
	Enabled        bool
	Profile        string
	TargetNodeID   string
	DropPct        int
	DelayMsMin     int
	DelayMsMax     int
	Seed           int64
	WindowMs       int
	ExitOnLeader   bool
}

// LoadConfig loads chaos configuration from environment variables
func LoadConfig() *Config {
	enabled := getEnvAsBool("CHAOS_ENABLED", false)
	profile := getEnvAsString("CHAOS_PROFILE", "")
	targetNodeID := getEnvAsString("CHAOS_TARGET_NODE_ID", "")
	dropPct := getEnvAsInt("CHAOS_DROP_PCT", 0)
	delayMsMin := getEnvAsInt("CHAOS_DELAY_MS_MIN", 0)
	delayMsMax := getEnvAsInt("CHAOS_DELAY_MS_MAX", 0)
	seed := getEnvAsInt64("CHAOS_SEED", 1)
	windowMs := getEnvAsInt("CHAOS_WINDOW_MS", 0)
	exitOnLeader := getEnvAsBool("CHAOS_EXIT_ON_LEADER", false)

	return &Config{
		Enabled:      enabled,
		Profile:      profile,
		TargetNodeID: targetNodeID,
		DropPct:      dropPct,
		DelayMsMin:   delayMsMin,
		DelayMsMax:   delayMsMax,
		Seed:         seed,
		WindowMs:     windowMs,
		ExitOnLeader: exitOnLeader,
	}
}

// ParseProfile parses a profile string like "drop-pct=30,delay=50-250"
func ParseProfile(profile string) (dropPct int, delayMin int, delayMax int, err error) {
	if profile == "" {
		return 0, 0, 0, nil
	}

	parts := strings.Split(profile, ",")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if strings.HasPrefix(part, "drop-pct=") {
			val := strings.TrimPrefix(part, "drop-pct=")
			dropPct, err = strconv.Atoi(val)
			if err != nil {
				return 0, 0, 0, fmt.Errorf("invalid drop-pct: %w", err)
			}
		} else if strings.HasPrefix(part, "delay=") {
			val := strings.TrimPrefix(part, "delay=")
			delayParts := strings.Split(val, "-")
			if len(delayParts) == 2 {
				delayMin, err = strconv.Atoi(delayParts[0])
				if err != nil {
					return 0, 0, 0, fmt.Errorf("invalid delay min: %w", err)
				}
				delayMax, err = strconv.Atoi(delayParts[1])
				if err != nil {
					return 0, 0, 0, fmt.Errorf("invalid delay max: %w", err)
				}
			}
		}
	}

	return dropPct, delayMin, delayMax, nil
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

func getEnvAsInt64(key string, defaultValue int64) int64 {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.ParseInt(value, 10, 64); err == nil {
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


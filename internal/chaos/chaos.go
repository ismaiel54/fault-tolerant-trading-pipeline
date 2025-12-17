package chaos

import (
	"context"
	"math/rand"
	"sync"
	"time"

	"go.uber.org/zap"
)

// Chaos provides deterministic failure injection
type Chaos struct {
	cfg    *Config
	logger *zap.Logger
	rng    *rand.Rand
	mu     sync.Mutex
	start  time.Time
}

// New creates a new Chaos instance
func New(cfg *Config, logger *zap.Logger) *Chaos {
	c := &Chaos{
		cfg:    cfg,
		logger: logger,
		rng:    rand.New(rand.NewSource(cfg.Seed)),
		start:  time.Now(),
	}

	// Apply profile if set
	if cfg.Profile != "" {
		dropPct, delayMin, delayMax, err := ParseProfile(cfg.Profile)
		if err != nil {
			logger.Warn("failed to parse chaos profile", zap.Error(err))
		} else {
			if dropPct > 0 {
				cfg.DropPct = dropPct
			}
			if delayMin > 0 || delayMax > 0 {
				cfg.DelayMsMin = delayMin
				cfg.DelayMsMax = delayMax
			}
		}
	}

	return c
}

// EnabledFor checks if chaos is enabled for a specific node
func (c *Chaos) EnabledFor(nodeID string) bool {
	if !c.cfg.Enabled {
		return false
	}

	// Check if window expired
	if c.cfg.WindowMs > 0 {
		elapsed := time.Since(c.start).Milliseconds()
		if elapsed > int64(c.cfg.WindowMs) {
			return false
		}
	}

	// If target node is set, only apply to that node
	if c.cfg.TargetNodeID != "" && c.cfg.TargetNodeID != nodeID {
		return false
	}

	return true
}

// MaybeDelay injects a random delay if chaos is enabled
func (c *Chaos) MaybeDelay(ctx context.Context, nodeID, op string) error {
	if !c.EnabledFor(nodeID) {
		return nil
	}

	if c.cfg.DelayMsMin == 0 && c.cfg.DelayMsMax == 0 {
		return nil
	}

	c.mu.Lock()
	var delayMs int
	if c.cfg.DelayMsMin == c.cfg.DelayMsMax {
		delayMs = c.cfg.DelayMsMin
	} else {
		delayMs = c.cfg.DelayMsMin + c.rng.Intn(c.cfg.DelayMsMax-c.cfg.DelayMsMin+1)
	}
	c.mu.Unlock()

	if delayMs > 0 {
		c.logger.Info("chaos delay injected",
			zap.String("node_id", nodeID),
			zap.String("op", op),
			zap.Int("delay_ms", delayMs),
		)

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Duration(delayMs) * time.Millisecond):
			return nil
		}
	}

	return nil
}

// MaybeDrop returns true if the request should be dropped
func (c *Chaos) MaybeDrop(nodeID, op string) bool {
	if !c.EnabledFor(nodeID) {
		return false
	}

	if c.cfg.DropPct == 0 {
		return false
	}

	c.mu.Lock()
	drop := c.rng.Intn(100) < c.cfg.DropPct
	c.mu.Unlock()

	if drop {
		c.logger.Info("chaos drop injected",
			zap.String("node_id", nodeID),
			zap.String("op", op),
			zap.Bool("dropped", true),
		)
	}

	return drop
}

// ShouldExitOnLeader returns true if node should exit when becoming leader
func (c *Chaos) ShouldExitOnLeader() bool {
	return c.cfg.ExitOnLeader
}


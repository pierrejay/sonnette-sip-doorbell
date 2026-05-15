package main

import (
	"sync"
	"time"
)

type Cooldown struct {
	mu          sync.Mutex
	cooldownDur time.Duration
	maxPerHour  int
	lastPress   time.Time
	hourLog     []time.Time
}

func NewCooldown(cooldownSec, maxPerHour int) *Cooldown {
	return &Cooldown{
		cooldownDur: time.Duration(cooldownSec) * time.Second,
		maxPerHour:  maxPerHour,
	}
}

// Allow returns true if a new call is permitted.
func (c *Cooldown) Allow() bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()

	// Cooldown since last press
	if !c.lastPress.IsZero() && now.Sub(c.lastPress) < c.cooldownDur {
		return false
	}

	// Max calls per hour
	cutoff := now.Add(-time.Hour)
	filtered := c.hourLog[:0]
	for _, t := range c.hourLog {
		if t.After(cutoff) {
			filtered = append(filtered, t)
		}
	}
	c.hourLog = filtered

	if len(c.hourLog) >= c.maxPerHour {
		return false
	}

	c.lastPress = now
	c.hourLog = append(c.hourLog, now)
	return true
}

// Remaining returns seconds until cooldown expires. 0 if ready.
func (c *Cooldown) Remaining() int {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.lastPress.IsZero() {
		return 0
	}
	r := c.cooldownDur - time.Since(c.lastPress)
	if r <= 0 {
		return 0
	}
	return int(r.Seconds()) + 1
}

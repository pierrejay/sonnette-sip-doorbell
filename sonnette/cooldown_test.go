package main

import (
	"testing"
	"time"
)

func TestCooldown_FirstPressAllowed(t *testing.T) {
	cd := NewCooldown(30, 10)
	if !cd.Allow() {
		t.Fatal("first press should be allowed")
	}
}

func TestCooldown_SecondPressBlocked(t *testing.T) {
	cd := NewCooldown(30, 10)
	cd.Allow()
	if cd.Allow() {
		t.Fatal("second press within cooldown should be blocked")
	}
}

func TestCooldown_AllowedAfterExpiry(t *testing.T) {
	cd := NewCooldown(1, 10) // 1 second cooldown
	cd.Allow()
	time.Sleep(1100 * time.Millisecond)
	if !cd.Allow() {
		t.Fatal("press after cooldown expiry should be allowed")
	}
}

func TestCooldown_MaxPerHour(t *testing.T) {
	cd := NewCooldown(0, 3) // no cooldown, max 3/hr

	for i := 0; i < 3; i++ {
		if !cd.Allow() {
			t.Fatalf("press %d should be allowed", i+1)
		}
	}
	if cd.Allow() {
		t.Fatal("press beyond max/hr should be blocked")
	}
}

func TestCooldown_Remaining(t *testing.T) {
	cd := NewCooldown(30, 10)

	if cd.Remaining() != 0 {
		t.Fatal("remaining should be 0 before any press")
	}

	cd.Allow()
	r := cd.Remaining()
	if r < 29 || r > 31 {
		t.Fatalf("remaining should be ~30, got %d", r)
	}
}

func TestCooldown_HourWindowSlides(t *testing.T) {
	cd := &Cooldown{
		cooldownDur: 0,
		maxPerHour:  2,
	}
	// Simulate old presses outside the 1-hour window
	cd.hourLog = []time.Time{
		time.Now().Add(-2 * time.Hour),
		time.Now().Add(-90 * time.Minute),
	}
	if !cd.Allow() {
		t.Fatal("old presses outside window should not count")
	}
}

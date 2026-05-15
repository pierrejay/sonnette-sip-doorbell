package main

import (
	"sync/atomic"
	"time"
)

// RingState is a snapshot of the last ring activity, exposed via the
// /health endpoint. Once published it is immutable — RecordX builds a
// new struct and atomically swaps the pointer, so readers never see a
// half-updated struct and never block.
type RingState struct {
	LastButtonPress   time.Time
	LastCallInitiated time.Time
	LastCallSID       string
}

var ringState atomic.Pointer[RingState]

// RecordButtonPress notes that the button was pressed (even if the
// press is later blocked by cooldown or by pjsua being down — this
// field reflects "was anyone there?" not "did a call go through?").
func RecordButtonPress() {
	old := ringState.Load()
	next := RingState{LastButtonPress: time.Now()}
	if old != nil {
		next.LastCallInitiated = old.LastCallInitiated
		next.LastCallSID = old.LastCallSID
	}
	ringState.Store(&next)
}

// RecordCallInitiated notes a successful Twilio REST API call (the
// SID is the one returned by Twilio).
func RecordCallInitiated(sid string) {
	old := ringState.Load()
	next := RingState{
		LastCallInitiated: time.Now(),
		LastCallSID:       sid,
	}
	if old != nil {
		next.LastButtonPress = old.LastButtonPress
	}
	ringState.Store(&next)
}

// LoadRingState returns the current snapshot (or nil if no event has
// been recorded yet).
func LoadRingState() *RingState {
	return ringState.Load()
}

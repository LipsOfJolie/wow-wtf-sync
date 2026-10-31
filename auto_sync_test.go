package main

import (
	"reflect"
	"testing"
	"time"
)

func TestProbeGitHubWithRetries(t *testing.T) {
	attempts := 0
	var waits []time.Duration
	reachable := probeGitHubWithRetries(func() bool {
		attempts++
		return attempts == 3
	}, 3, 5*time.Second, func(duration time.Duration) {
		waits = append(waits, duration)
	})
	if !reachable {
		t.Fatal("expected third connection attempt to succeed")
	}
	if attempts != 3 {
		t.Fatalf("probe attempts = %d, want 3", attempts)
	}
	if want := []time.Duration{5 * time.Second, 5 * time.Second}; !reflect.DeepEqual(waits, want) {
		t.Errorf("retry waits = %v, want %v", waits, want)
	}
}

func TestProbeGitHubWithRetriesStopsAfterFailureLimit(t *testing.T) {
	attempts := 0
	if probeGitHubWithRetries(func() bool {
		attempts++
		return false
	}, 3, time.Second, func(time.Duration) {}) {
		t.Fatal("offline probe unexpectedly succeeded")
	}
	if attempts != 3 {
		t.Fatalf("probe attempts = %d, want 3", attempts)
	}
}

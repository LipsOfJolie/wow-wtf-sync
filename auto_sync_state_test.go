package main

import (
	"testing"
	"time"
)

func TestAutoSyncStateRunsOnceOnStartup(t *testing.T) {
	state := autoSyncState{startupPending: true}
	now := time.Now()

	if !state.update(true, false, now) {
		t.Fatal("startup with no running game should trigger synchronization")
	}
	if state.update(true, false, now.Add(time.Second)) {
		t.Fatal("startup synchronization should only run once")
	}
}

func TestAutoSyncStateWaitsForGitHubBeforeStartupSync(t *testing.T) {
	state := autoSyncState{startupPending: true}
	now := time.Now()

	if state.updateWithGitHubAvailability(true, false, false, now) {
		t.Fatal("startup synchronization should wait until GitHub is reachable")
	}
	if !state.startupPending {
		t.Fatal("startup synchronization should remain pending while GitHub is unreachable")
	}
	if !state.updateWithGitHubAvailability(true, false, true, now.Add(githubRetryInterval)) {
		t.Fatal("startup synchronization should run when GitHub becomes reachable")
	}
}

func TestAutoSyncStateRunsOnStartupEvenWhenGameIsOpen(t *testing.T) {
	state := autoSyncState{startupPending: true}
	now := time.Now()

	if !state.update(true, true, now) {
		t.Fatal("application startup should trigger synchronization")
	}
	if state.update(true, true, now.Add(time.Second)) {
		t.Fatal("startup synchronization should only run once")
	}
	if state.update(true, false, now.Add(2*time.Second)) {
		t.Fatal("game-close synchronization should still wait for the grace period")
	}
	if !state.update(true, false, now.Add(2*time.Second+gameCloseGracePeriod)) {
		t.Fatal("closing the game should trigger a separate synchronization after the grace period")
	}
}

func TestAutoSyncStateWaitsForAllGameProcessesToClose(t *testing.T) {
	state := autoSyncState{}
	now := time.Now()

	if state.update(true, true, now) {
		t.Fatal("running game should not trigger synchronization")
	}
	if state.update(true, true, now.Add(time.Minute)) {
		t.Fatal("another running game process should keep synchronization pending")
	}
	if state.update(true, false, now.Add(2*time.Minute)) {
		t.Fatal("synchronization should wait after the last process exits")
	}
	if state.update(true, false, now.Add(2*time.Minute+gameCloseGracePeriod-time.Nanosecond)) {
		t.Fatal("synchronization should wait for the full grace period")
	}
	if !state.update(true, false, now.Add(2*time.Minute+gameCloseGracePeriod)) {
		t.Fatal("synchronization should run after the grace period with no process")
	}
}

func TestAutoSyncStateCancelsCloseTimerWhenGameRestarts(t *testing.T) {
	state := autoSyncState{}
	now := time.Now()

	state.update(true, true, now)
	state.update(true, false, now.Add(time.Second))
	if state.update(true, true, now.Add(2*time.Second)) {
		t.Fatal("new game process should cancel the pending close synchronization")
	}
	state.update(true, false, now.Add(3*time.Second))
	if state.update(true, false, now.Add(6*time.Second-time.Nanosecond)) {
		t.Fatal("grace period should restart after the final process exits")
	}
	if !state.update(true, false, now.Add(6*time.Second)) {
		t.Fatal("synchronization should run after the restarted grace period")
	}
}

func TestAutoSyncStateDoesNotTriggerWhenEnabledAfterStartup(t *testing.T) {
	state := autoSyncState{}
	if state.update(true, false, time.Now()) {
		t.Fatal("enabling auto-sync after startup should not itself trigger synchronization")
	}
}

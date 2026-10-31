package main

import (
	"net/http"
	"time"
)

const gameCloseGracePeriod = 3 * time.Second
const githubRetryInterval = 10 * time.Second
const githubStartupAttempts = 3
const githubStartupRetryInterval = 5 * time.Second

type automaticSyncAction string

const (
	autoSyncPush          automaticSyncAction = "push"
	autoSyncPull          automaticSyncAction = "pull"
	autoSyncAlreadySynced automaticSyncAction = "synced"
	autoSyncConflict      automaticSyncAction = "conflict"
)

type autoSyncState struct {
	startupPending bool
	wasRunning     bool
	syncAfter      time.Time
}

func (s *autoSyncState) update(enabled, running bool, now time.Time) bool {
	return s.updateWithGitHubAvailability(enabled, running, true, now)
}

func (s *autoSyncState) updateWithGitHubAvailability(enabled, running, githubReady bool, now time.Time) bool {
	if !enabled {
		s.startupPending = false
		s.wasRunning = false
		s.syncAfter = time.Time{}
		return false
	}
	if s.startupPending && !githubReady {
		return false
	}
	if s.startupPending {
		s.startupPending = false
		s.wasRunning = running
		return true
	}
	if running {
		s.wasRunning = true
		s.syncAfter = time.Time{}
		return false
	}
	if s.wasRunning {
		s.wasRunning = false
		s.syncAfter = now.Add(gameCloseGracePeriod)
		return false
	}
	if !s.syncAfter.IsZero() {
		if now.Before(s.syncAfter) {
			return false
		}
		s.syncAfter = time.Time{}
		s.startupPending = false
		return true
	}
	return false
}

func monitorAutoSyncWithProbe(getConfig func() Config, synchronize func(), setStatus func(string), text uiText, probe func() (bool, error), runOnStartup bool, githubReady <-chan struct{}) {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	state := autoSyncState{startupPending: runOnStartup}
	var lastProcessError string

	for range ticker.C {
		cfg := getConfig()
		if !cfg.AutoSync {
			state.update(false, false, time.Now())
			continue
		}
		if cfg.GamePath == "" || cfg.RepoURL == "" || cfg.GitHubToken == "" {
			continue
		}

		isGitHubReady := true
		if state.startupPending {
			select {
			case <-githubReady:
			default:
				isGitHubReady = false
			}
			if !isGitHubReady {
				continue
			}
		}

		running, err := probe()
		if err != nil {
			if err.Error() != lastProcessError {
				setStatus(text.processCheckError + err.Error())
				lastProcessError = err.Error()
			}
			continue
		}
		lastProcessError = ""

		if state.updateWithGitHubAvailability(cfg.AutoSync, running, isGitHubReady, time.Now()) {
			setStatus(text.autoSyncChecking)
			go synchronize()
		}
	}
}

func monitorGitHubAvailability(getConfig func() Config, ready chan<- struct{}, onReady func(), setWaiting func(), probe func() bool) {
	ticker := time.NewTicker(githubRetryInterval)
	defer ticker.Stop()

	initialAttemptsDone := false
	for {
		cfg := getConfig()
		if cfg.GamePath == "" || cfg.RepoURL == "" || cfg.GitHubToken == "" {
			<-ticker.C
			continue
		}
		if !initialAttemptsDone {
			initialAttemptsDone = true
			if probeGitHubWithRetries(probe, githubStartupAttempts, githubStartupRetryInterval, waitForRetry) {
				close(ready)
				onReady()
				return
			}
			setWaiting()
			continue
		}
		if probe() {
			close(ready)
			onReady()
			return
		}
		setWaiting()
		<-ticker.C
	}
}

func probeGitHubWithRetries(probe func() bool, attempts int, interval time.Duration, wait func(time.Duration)) bool {
	for attempt := 0; attempt < attempts; attempt++ {
		if probe() {
			return true
		}
		if attempt+1 < attempts {
			wait(interval)
		}
	}
	return false
}

func waitForRetry(interval time.Duration) {
	timer := time.NewTimer(interval)
	defer timer.Stop()
	<-timer.C
}

func isGitHubReachable() bool {
	client := http.Client{Timeout: 3 * time.Second}
	response, err := client.Get("https://api.github.com/rate_limit")
	if err != nil {
		return false
	}
	if err := response.Body.Close(); err != nil {
		return false
	}
	return response.StatusCode < http.StatusInternalServerError
}

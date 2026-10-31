//go:build windows

package main

import "testing"

func TestIsGameRunningCanEnumerateWindowsProcesses(t *testing.T) {
	if _, err := isGameRunning(); err != nil {
		t.Fatalf("isGameRunning() returned an error: %v", err)
	}
}

//go:build !windows

package main

import (
	"fmt"
	"os/exec"
	"runtime"
)

func openBackupDirectory(path string) error {
	command := "xdg-open"
	if runtime.GOOS == "darwin" {
		command = "open"
	}
	if err := exec.Command(command, path).Start(); err != nil {
		return fmt.Errorf("open backup directory: %w", err)
	}
	return nil
}

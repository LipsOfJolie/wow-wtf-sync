//go:build windows

package main

import (
	"fmt"
	"os/exec"
)

func openBackupDirectory(path string) error {
	if err := exec.Command("explorer.exe", path).Start(); err != nil {
		return fmt.Errorf("start File Explorer: %w", err)
	}
	return nil
}

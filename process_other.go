//go:build !windows

package main

import (
	"errors"
	"fmt"
	"os/exec"
)

func isGameRunning() (bool, error) {
	err := exec.Command("pgrep", "-x", "wow.exe").Run()
	if err == nil {
		return true, nil
	}
	var exitError *exec.ExitError
	if errors.As(err, &exitError) && exitError.ExitCode() == 1 {
		return false, nil
	}
	return false, fmt.Errorf("не удалось проверить процесс wow.exe: %w", err)
}

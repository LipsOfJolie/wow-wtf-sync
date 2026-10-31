//go:build windows

package main

import (
	"fmt"
	"os"
	"strings"

	"golang.org/x/sys/windows/registry"
)

const windowsRunKey = `Software\Microsoft\Windows\CurrentVersion\Run`
const windowsRunValue = "WoWWTFSync"

func setAutostart(enabled bool) error {
	key, _, err := registry.CreateKey(registry.CURRENT_USER, windowsRunKey, registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("не удалось открыть настройки автозапуска Windows: %w", err)
	}
	defer key.Close()

	if !enabled {
		err := key.DeleteValue(windowsRunValue)
		if err != nil && err != registry.ErrNotExist {
			return fmt.Errorf("не удалось отключить автозапуск Windows: %w", err)
		}
		return nil
	}

	executable, err := os.Executable()
	if err != nil {
		return fmt.Errorf("не удалось определить путь к приложению: %w", err)
	}
	command := `"` + strings.ReplaceAll(executable, `"`, `\"`) + `"`
	if err := key.SetStringValue(windowsRunValue, command); err != nil {
		return fmt.Errorf("не удалось включить автозапуск Windows: %w", err)
	}
	return nil
}

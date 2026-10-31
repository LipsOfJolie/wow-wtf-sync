//go:build windows

package main

import (
	"errors"
	"fmt"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
)

func isGameRunning() (bool, error) {
	snapshot, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return false, fmt.Errorf("не удалось получить список процессов Windows: %w", err)
	}
	defer windows.CloseHandle(snapshot)

	entry := windows.ProcessEntry32{Size: uint32(unsafe.Sizeof(windows.ProcessEntry32{}))}
	err = windows.Process32First(snapshot, &entry)
	if errors.Is(err, windows.ERROR_NO_MORE_FILES) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("не удалось прочитать список процессов Windows: %w", err)
	}

	for {
		if strings.EqualFold(windows.UTF16ToString(entry.ExeFile[:]), "wow.exe") {
			return true, nil
		}
		err = windows.Process32Next(snapshot, &entry)
		if errors.Is(err, windows.ERROR_NO_MORE_FILES) {
			return false, nil
		}
		if err != nil {
			return false, fmt.Errorf("не удалось продолжить чтение списка процессов Windows: %w", err)
		}
	}
}

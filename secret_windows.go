//go:build windows

package main

import (
	"encoding/base64"
	"errors"
	"fmt"
	"runtime"
	"unsafe"

	"golang.org/x/sys/windows"
)

func protectSecret(value string) (string, error) {
	input := []byte(value)
	inputBlob := windows.DataBlob{Size: uint32(len(input)), Data: &input[0]}
	var output windows.DataBlob
	if err := windows.CryptProtectData(&inputBlob, nil, nil, 0, nil, 1, &output); err != nil {
		return "", fmt.Errorf("DPAPI encryption failed: %w", err)
	}
	defer windows.LocalFree(windows.Handle(unsafe.Pointer(output.Data)))
	runtime.KeepAlive(input)
	protected := unsafe.Slice(output.Data, output.Size)
	return base64.StdEncoding.EncodeToString(protected), nil
}

func unprotectSecret(value string) (string, error) {
	protected, err := base64.StdEncoding.DecodeString(value)
	if err != nil || len(protected) == 0 {
		return "", errors.New("stored token data is invalid")
	}
	inputBlob := windows.DataBlob{Size: uint32(len(protected)), Data: &protected[0]}
	var output windows.DataBlob
	if err := windows.CryptUnprotectData(&inputBlob, nil, nil, 0, nil, 1, &output); err != nil {
		return "", fmt.Errorf("DPAPI decryption failed: %w", err)
	}
	defer windows.LocalFree(windows.Handle(unsafe.Pointer(output.Data)))
	runtime.KeepAlive(protected)
	return string(unsafe.Slice(output.Data, output.Size)), nil
}

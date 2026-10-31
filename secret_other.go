//go:build !windows

package main

import "errors"

func protectSecret(string) (string, error) {
	return "", errors.New("secure token storage is only supported on Windows")
}

func unprotectSecret(string) (string, error) {
	return "", errors.New("secure token storage is only supported on Windows")
}

//go:build !windows

package main

import "os"

func isReparsePoint(os.FileInfo) bool {
	return false
}

//go:build !windows

package main

import "errors"

func setAutostart(bool) error {
	return errors.New("автозапуск поддерживается только в Windows")
}

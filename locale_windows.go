//go:build windows

package main

import "golang.org/x/sys/windows"

func systemLanguage() string {
	languages, err := windows.GetUserPreferredUILanguages(windows.MUI_LANGUAGE_NAME)
	if err != nil {
		return "en"
	}

	if len(languages) > 0 && isRussianLocale(languages[0]) {
		return "ru"
	}
	return "en"
}

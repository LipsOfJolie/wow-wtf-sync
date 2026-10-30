package main

import (
	"reflect"
	"testing"
)

func TestIsRussianLocale(t *testing.T) {
	tests := []struct {
		language string
		want     bool
	}{
		{language: "ru", want: true},
		{language: "ru-RU", want: true},
		{language: "RU-ru", want: true},
		{language: "en-US", want: false},
		{language: "fr-FR", want: false},
		{language: "", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.language, func(t *testing.T) {
			if got := isRussianLocale(tt.language); got != tt.want {
				t.Errorf("isRussianLocale(%q) = %t, want %t", tt.language, got, tt.want)
			}
		})
	}
}

func TestTranslationsAreComplete(t *testing.T) {
	for _, language := range []string{"ru", "en"} {
		translation := textForLanguage(language)
		value := reflect.ValueOf(translation)
		for i := 0; i < value.NumField(); i++ {
			if value.Field(i).String() == "" {
				t.Errorf("%s translation is missing %s", language, value.Type().Field(i).Name)
			}
		}
	}
}

func TestUnknownLanguageUsesEnglish(t *testing.T) {
	if got, want := textForLanguage("fr").statusSynced, textForLanguage("en").statusSynced; got != want {
		t.Errorf("unsupported language selected %q, want English translation %q", got, want)
	}
}

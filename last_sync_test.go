package main

import (
	"testing"
	"time"
)

func TestFormatLastSyncShowsResultBeforeDate(t *testing.T) {
	text := textForLanguage("ru")
	when := time.Date(2026, time.October, 31, 10, 12, 0, 0, time.Local)
	got := formatLastSync(when.Format(time.RFC3339), text.statusSynced, text)
	want := "Синхронизация не требуется. Последняя синхронизация: 31.10.2026 10:12"
	if got != want {
		t.Errorf("formatLastSync() = %q, want %q", got, want)
	}
}

func TestFormatLastSyncShowsNeverWhenNoSyncWasRecorded(t *testing.T) {
	text := textForLanguage("ru")
	if got := formatLastSync("", "", text); got != text.lastSyncNever {
		t.Errorf("formatLastSync() = %q, want %q", got, text.lastSyncNever)
	}
}

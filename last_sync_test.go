package main

import (
	"testing"
	"time"
)

func TestFormatLastSyncShowsResultBeforeDate(t *testing.T) {
	text := textForLanguage("ru")
	when := time.Date(2026, time.October, 31, 10, 12, 0, 0, time.Local)
	got := formatLastSync(when.Format(time.RFC3339), text.statusSynced, text)
	want := "Проверка завершена: синхронизация не потребовалась. Последняя проверка: 31.10.2026 10:12"
	if got != want {
		t.Errorf("formatLastSync() = %q, want %q", got, want)
	}
}

func TestFormatLastSyncDistinguishesSuccessfulSync(t *testing.T) {
	text := textForLanguage("ru")
	when := time.Date(2026, time.October, 31, 10, 12, 0, 0, time.Local)
	got := formatLastSync(when.Format(time.RFC3339), text.lastPushSuccess, text)
	want := "Успешный Push. Последняя синхронизация: 31.10.2026 10:12"
	if got != want {
		t.Errorf("formatLastSync() = %q, want %q", got, want)
	}
}

func TestFormatLastSyncShowsNeverWhenNoSyncWasRecorded(t *testing.T) {
	text := textForLanguage("ru")
	if got := formatLastSync("", "", text); got != text.lastActivityNever {
		t.Errorf("formatLastSync() = %q, want %q", got, text.lastActivityNever)
	}
}

func TestFormatStatusWithHistoryCombinesCurrentAndPreviousStatus(t *testing.T) {
	text := textForLanguage("ru")
	when := time.Date(2026, time.October, 31, 10, 12, 0, 0, time.Local)
	got := formatStatusWithHistory(text.syncFailed, when.Format(time.RFC3339), text.lastPullSuccess, text)
	want := "❌ Ошибка синхронизации: \nУспешный Pull. Последняя синхронизация: 31.10.2026 10:12"
	if got != want {
		t.Errorf("formatStatusWithHistory() = %q, want %q", got, want)
	}
}

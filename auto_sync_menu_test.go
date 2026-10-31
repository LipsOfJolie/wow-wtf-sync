package main

import "testing"

func TestAutoSyncMenuLabel(t *testing.T) {
	tests := []struct {
		enabled bool
		want    string
	}{
		{enabled: true, want: "☑ Automatic sync"},
		{enabled: false, want: "☐ Automatic sync"},
	}

	for _, tt := range tests {
		if got := autoSyncMenuLabel(tt.enabled, "Automatic sync"); got != tt.want {
			t.Errorf("autoSyncMenuLabel(%t) = %q, want %q", tt.enabled, got, tt.want)
		}
	}
}

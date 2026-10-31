package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestPruneBackupArchivesKeepsNewestAndIgnoresOtherFiles(t *testing.T) {
	dir := t.TempDir()
	base := time.Now().Add(-time.Hour)
	for i := 0; i < 7; i++ {
		path := filepath.Join(dir, "SavedVariables_Backup_"+string(rune('0'+i))+".zip")
		if err := os.WriteFile(path, []byte("backup"), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(path, base.Add(time.Duration(i)*time.Minute), base.Add(time.Duration(i)*time.Minute)); err != nil {
			t.Fatal(err)
		}
	}
	unrelated := filepath.Join(dir, "other.zip")
	if err := os.WriteFile(unrelated, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := pruneBackupArchives(dir, maxBackupArchives); err != nil {
		t.Fatal(err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := len(entries), maxBackupArchives+1; got != want {
		t.Fatalf("remaining file count = %d, want %d", got, want)
	}
	for i := 0; i < 7-maxBackupArchives; i++ {
		path := filepath.Join(dir, "SavedVariables_Backup_"+string(rune('0'+i))+".zip")
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Errorf("old backup %q was not removed", path)
		}
	}
	if _, err := os.Stat(unrelated); err != nil {
		t.Errorf("unrelated archive was removed: %v", err)
	}
}

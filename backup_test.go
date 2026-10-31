package main

import (
	"archive/zip"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestBackupDirectoryForExecutable(t *testing.T) {
	executablePath := filepath.Join(t.TempDir(), "WoWWTFSync.exe")
	want := filepath.Join(filepath.Dir(executablePath), "backups")

	if got := backupDirectoryForExecutable(executablePath); got != want {
		t.Fatalf("backupDirectoryForExecutable() = %q, want %q", got, want)
	}
}

func TestBackupArchiveContainsOnlySavedVariables(t *testing.T) {
	accountPath := t.TempDir()
	savedVariablesFile := filepath.Join(accountPath, "Realm", "SavedVariables", "addon.lua")
	unrelatedFile := filepath.Join(accountPath, "Realm", "other.lua")
	for path, content := range map[string]string{
		savedVariablesFile: "saved",
		unrelatedFile:      "unrelated",
	} {
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	archivePath := filepath.Join(t.TempDir(), "backup.zip")
	if err := createBackupArchive(accountPath, archivePath); err != nil {
		t.Fatal(err)
	}
	archive, err := zip.OpenReader(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	defer archive.Close()
	if len(archive.File) != 4 {
		t.Fatalf("archive contains %d entries, want all selected-folder directories and files", len(archive.File))
	}
	names := make(map[string]bool)
	for _, entry := range archive.File {
		names[strings.TrimSuffix(filepath.ToSlash(entry.Name), "/")] = true
	}
	if !names["Realm/other.lua"] || !names["Realm/SavedVariables/addon.lua"] {
		t.Errorf("archive does not contain all selected-folder files: %v", names)
	}
}

func TestInterruptedBackupIsNotPublished(t *testing.T) {
	archivePath := filepath.Join(t.TempDir(), "backup.zip")
	err := createBackupArchive(filepath.Join(t.TempDir(), "missing-source"), archivePath)
	if err == nil {
		t.Fatal("backup unexpectedly succeeded for a missing source directory")
	}
	if _, statErr := os.Stat(archivePath); !os.IsNotExist(statErr) {
		t.Fatalf("incomplete archive was published: %v", statErr)
	}
	entries, err := os.ReadDir(filepath.Dir(archivePath))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("temporary archive was not cleaned up: %v", entries)
	}
}

func TestRestoreArchiveOverlaysOnlySavedVariables(t *testing.T) {
	accountPath := t.TempDir()
	paths := map[string]string{
		filepath.Join("Realm", "SavedVariables", "addon.lua"): "old",
		filepath.Join("Realm", "SavedVariables", "local.lua"): "preserve",
		filepath.Join("Realm", "other.lua"):                   "unrelated",
	}
	for path, content := range paths {
		fullPath := filepath.Join(accountPath, path)
		if err := os.MkdirAll(filepath.Dir(fullPath), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(fullPath, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	archivePath := filepath.Join(t.TempDir(), "restore.zip")
	createTestArchive(t, archivePath, map[string]string{
		"Realm/SavedVariables/addon.lua": "restored",
	})

	if err := restoreSavedVariablesFromArchive(archivePath, accountPath); err != nil {
		t.Fatal(err)
	}
	for path, want := range map[string]string{
		filepath.Join("Realm", "SavedVariables", "addon.lua"): "restored",
		filepath.Join("Realm", "SavedVariables", "local.lua"): "preserve",
		filepath.Join("Realm", "other.lua"):                   "unrelated",
	} {
		got, err := os.ReadFile(filepath.Join(accountPath, path))
		if err != nil {
			t.Fatalf("read %q: %v", path, err)
		}
		if string(got) != want {
			t.Errorf("%q = %q, want %q", path, got, want)
		}
	}
}

func TestRestoreArchiveRejectsTraversalPath(t *testing.T) {
	archivePath := filepath.Join(t.TempDir(), "bad.zip")
	createTestArchive(t, archivePath, map[string]string{
		"../Realm/SavedVariables/evil.lua": "bad",
	})
	if err := restoreSavedVariablesFromArchive(archivePath, t.TempDir()); err == nil {
		t.Fatal("archive traversal path was accepted")
	}
}

func TestBackupRotationKeepsNewestArchives(t *testing.T) {
	directory := t.TempDir()
	base := time.Unix(1_700_000_000, 0)
	for index := 0; index < 7; index++ {
		path := filepath.Join(directory, "SavedVariables_Backup_"+string(rune('0'+index))+".zip")
		if err := os.WriteFile(path, []byte("archive"), 0o600); err != nil {
			t.Fatal(err)
		}
		modTime := base.Add(time.Duration(index) * time.Minute)
		if err := os.Chtimes(path, modTime, modTime); err != nil {
			t.Fatal(err)
		}
	}

	if err := pruneBackupArchives(directory, maxBackupArchives); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(directory)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != maxBackupArchives {
		t.Fatalf("remaining archives = %d, want %d", len(entries), maxBackupArchives)
	}
	for index := 0; index < 2; index++ {
		path := filepath.Join(directory, "SavedVariables_Backup_"+string(rune('0'+index))+".zip")
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Errorf("old archive %q was not removed: %v", path, err)
		}
	}
}

func createTestArchive(t *testing.T, path string, files map[string]string) {
	t.Helper()
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	writer := zip.NewWriter(file)
	for name, content := range files {
		entry, err := writer.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := entry.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
		if err := writer.Close(); err != nil {
			t.Fatal(err)
		}
		if err := file.Close(); err != nil {
			t.Fatal(err)
		}
	}
}

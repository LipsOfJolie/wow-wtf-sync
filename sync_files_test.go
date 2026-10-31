package main

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
)

func TestDirectoriesHaveSameContentsIgnoresModificationTimes(t *testing.T) {
	first := t.TempDir()
	second := t.TempDir()
	firstFile := filepath.Join(first, "nested", "settings.lua")
	secondFile := filepath.Join(second, "nested", "settings.lua")
	for _, path := range []string{firstFile, secondFile} {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("settings"), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	if err := os.Chtimes(firstFile, time.Unix(100, 0), time.Unix(100, 0)); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(secondFile, time.Unix(200, 0), time.Unix(200, 0)); err != nil {
		t.Fatal(err)
	}

	same, err := directoriesHaveSameContents(first, second)
	if err != nil {
		t.Fatal(err)
	}
	if !same {
		t.Fatal("directories with identical file contents should compare equal despite different timestamps")
	}
}

func TestDirectoriesHaveSameContentsDetectsDifferences(t *testing.T) {
	first := t.TempDir()
	second := t.TempDir()
	for root, contents := range map[string]string{first: "local", second: "cloud"} {
		if err := os.WriteFile(filepath.Join(root, "settings.lua"), []byte(contents), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	same, err := directoriesHaveSameContents(first, second)
	if err != nil {
		t.Fatal(err)
	}
	if same {
		t.Fatal("directories with different file contents should compare unequal")
	}
}

func TestDirectoriesHaveSameContentsIgnoresRepositoryMetadata(t *testing.T) {
	first := t.TempDir()
	second := t.TempDir()
	for _, root := range []string{first, second} {
		if err := os.Mkdir(filepath.Join(root, ".git"), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, ".git", "config"), []byte("different metadata"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(first, "README.md"), []byte("first"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(second, "README.md"), []byte("second"), 0o600); err != nil {
		t.Fatal(err)
	}

	same, err := directoriesHaveSameContents(first, second)
	if err != nil {
		t.Fatal(err)
	}
	if !same {
		t.Fatal("ignored files and .git metadata should not affect the comparison")
	}
}

func TestCopySavedVariablesSkipsLargeFilesAndPreservesExistingCopy(t *testing.T) {
	src := t.TempDir()
	dst := t.TempDir()
	savedVariables := filepath.Join("Realm", "SavedVariables")
	srcSavedVariables := filepath.Join(src, savedVariables)
	dstSavedVariables := filepath.Join(dst, savedVariables)
	if err := os.MkdirAll(srcSavedVariables, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(dstSavedVariables, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(srcSavedVariables, "large.lua"), []byte("01234567890"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(srcSavedVariables, "small.lua"), []byte("small"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "other.lua"), []byte("unrelated"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dstSavedVariables, "large.lua"), []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := copySavedVariablesFiles(src, dst, 10); err != nil {
		t.Fatal(err)
	}

	large, err := os.ReadFile(filepath.Join(dstSavedVariables, "large.lua"))
	if err != nil {
		t.Fatal(err)
	}
	if got, want := string(large), "old"; got != want {
		t.Errorf("large file contents = %q, want existing copy %q", got, want)
	}
	small, err := os.ReadFile(filepath.Join(dstSavedVariables, "small.lua"))
	if err != nil {
		t.Fatal(err)
	}
	if got, want := string(small), "small"; got != want {
		t.Errorf("small file contents = %q, want %q", got, want)
	}
	if contents, err := os.ReadFile(filepath.Join(dst, "other.lua")); err != nil || string(contents) != "unrelated" {
		t.Errorf("file outside SavedVariables was not copied: contents=%q err=%v", contents, err)
	}
}

func TestSavedVariablesFileSizesIncludesSelectedFolderFiles(t *testing.T) {
	dir := t.TempDir()
	savedVariables := filepath.Join(dir, "Realm", "SavedVariables")
	if err := os.MkdirAll(savedVariables, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(savedVariables, "small.lua"), []byte("small"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(savedVariables, "large.lua"), []byte("01234567890"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "unrelated.lua"), []byte("outside"), 0o600); err != nil {
		t.Fatal(err)
	}
	size, oversized, err := savedVariablesFileSizes(dir, 10)
	if err != nil {
		t.Fatal(err)
	}
	if size != int64(len("small")+len("outside")) {
		t.Errorf("included size = %d, want %d", size, len("small")+len("outside"))
	}
	if len(oversized) != 1 || filepath.Base(oversized[0].Path) != "large.lua" || oversized[0].Size != 11 {
		t.Errorf("oversized files = %#v, want only large.lua at 11 bytes", oversized)
	}
}

func TestPullOverlayPreservesSkippedAndUntrackedFiles(t *testing.T) {
	repoPath := t.TempDir()
	repo, err := git.PlainInit(repoPath, false)
	if err != nil {
		t.Fatal(err)
	}
	tracked := map[string]string{
		filepath.Join("Realm", "SavedVariables", "tracked.lua"): "from repository",
		filepath.Join("Realm", "SavedVariables", "large.lua"):   strings.Repeat("x", 30),
		filepath.Join("Realm", "OtherData", "unrelated.lua"):    "from repository",
	}
	worktree, err := repo.Worktree()
	if err != nil {
		t.Fatal(err)
	}
	for path, content := range tracked {
		fullPath := filepath.Join(repoPath, path)
		if err := os.MkdirAll(filepath.Dir(fullPath), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(fullPath, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := worktree.Add(filepath.ToSlash(path)); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := worktree.Commit("test", &git.CommitOptions{
		Author: &object.Signature{Name: "Test", Email: "test@example.com", When: time.Now()},
	}); err != nil {
		t.Fatal(err)
	}

	gamePath := t.TempDir()
	savedVariablesPath := filepath.Join(gamePath, "Realm", "SavedVariables")
	if err := os.MkdirAll(savedVariablesPath, 0o700); err != nil {
		t.Fatal(err)
	}
	existingFiles := map[string]string{
		filepath.Join("Realm", "SavedVariables", "tracked.lua"):   "old version",
		filepath.Join("Realm", "SavedVariables", "large.lua"):     "keep large local file",
		filepath.Join("Realm", "SavedVariables", "untracked.lua"): "keep untracked",
		filepath.Join("Realm", "OtherData", "unrelated.lua"):      "from repository",
	}
	for path, content := range existingFiles {
		fullPath := filepath.Join(gamePath, path)
		if err := os.MkdirAll(filepath.Dir(fullPath), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(fullPath, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	if err := copyTrackedSavedVariables(repoPath, gamePath, 20); err != nil {
		t.Fatal(err)
	}

	for path, want := range map[string]string{
		filepath.Join("Realm", "SavedVariables", "tracked.lua"):   "from repository",
		filepath.Join("Realm", "SavedVariables", "large.lua"):     "keep large local file",
		filepath.Join("Realm", "SavedVariables", "untracked.lua"): "keep untracked",
		filepath.Join("Realm", "OtherData", "unrelated.lua"):      "from repository",
	} {
		got, err := os.ReadFile(filepath.Join(gamePath, path))
		if err != nil {
			t.Fatalf("read %q: %v", path, err)
		}
		if string(got) != want {
			t.Errorf("%q = %q, want %q", path, got, want)
		}
	}
}

func TestPullRemovalDeletesOnlyPreviouslyTrackedFiles(t *testing.T) {
	repoPath := t.TempDir()
	repo, err := git.PlainInit(repoPath, false)
	if err != nil {
		t.Fatal(err)
	}
	worktree, err := repo.Worktree()
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"tracked-delete.lua", "tracked-keep.lua"} {
		if err := os.WriteFile(filepath.Join(repoPath, path), []byte(path), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := worktree.Add(path); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := worktree.Commit("initial", &git.CommitOptions{
		Author: &object.Signature{Name: "Test", Email: "test@example.com", When: time.Now()},
	}); err != nil {
		t.Fatal(err)
	}

	destination := t.TempDir()
	for _, path := range []string{"tracked-delete.lua", "tracked-keep.lua", "untracked.lua"} {
		if err := os.WriteFile(filepath.Join(destination, path), []byte("keep"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	previousPaths, err := trackedRepositoryFiles(repoPath, 80*1024*1024)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := worktree.Remove("tracked-delete.lua"); err != nil {
		t.Fatal(err)
	}
	if _, err := worktree.Commit("delete one tracked file", &git.CommitOptions{
		Author: &object.Signature{Name: "Test", Email: "test@example.com", When: time.Now()},
	}); err != nil {
		t.Fatal(err)
	}

	if err := removeTrackedFilesDeletedFromRepository(destination, repoPath, previousPaths, 80*1024*1024); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(destination, "tracked-delete.lua")); !os.IsNotExist(err) {
		t.Fatalf("deleted tracked file remains: %v", err)
	}
	for _, path := range []string{"tracked-keep.lua", "untracked.lua"} {
		if _, err := os.Stat(filepath.Join(destination, path)); err != nil {
			t.Errorf("file %q should be preserved: %v", path, err)
		}
	}
}

func TestCopyFileInterruptedWritePreservesOriginal(t *testing.T) {
	directory := t.TempDir()
	destination := filepath.Join(directory, "settings.lua")
	if err := os.WriteFile(destination, []byte("original"), 0o600); err != nil {
		t.Fatal(err)
	}
	writeErr := os.ErrPermission
	err := copyFileAtomically(destination, 0o600, func(writer io.Writer) error {
		if _, err := writer.Write([]byte("partial")); err != nil {
			return err
		}
		return writeErr
	})
	if !errors.Is(err, writeErr) {
		t.Fatalf("copyFileAtomically error = %v, want %v", err, writeErr)
	}
	got, err := os.ReadFile(destination)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "original" {
		t.Fatalf("original destination was modified: %q", got)
	}
	entries, err := os.ReadDir(directory)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != filepath.Base(destination) {
		t.Fatalf("temporary files were not cleaned up: %v", entries)
	}
}

func TestPullStopsBeforeNetworkWhenGitWorktreeIsDirty(t *testing.T) {
	repoPath := t.TempDir()
	_, err := git.PlainInit(repoPath, false)
	if err != nil {
		t.Fatal(err)
	}
	filePath := filepath.Join(repoPath, "local-change.txt")
	if err := os.WriteFile(filePath, []byte("uncommitted"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := GitPull(repoPath, ""); err == nil || !strings.Contains(err.Error(), "uncommitted changes") {
		t.Fatalf("GitPull error = %v, want dirty-worktree rejection", err)
	}
}

func TestPullBlockedWhileGameIsRunning(t *testing.T) {
	err := requireGameStopped(func() (bool, error) { return true, nil })
	if err == nil || !strings.Contains(err.Error(), "close wow.exe") {
		t.Fatalf("running-game guard error = %v", err)
	}
}

func TestSavedVariablesRootCanBeSelectedDirectly(t *testing.T) {
	gamePath := filepath.Join(t.TempDir(), "SavedVariables")
	if err := os.MkdirAll(gamePath, 0o700); err != nil {
		t.Fatal(err)
	}
	filePath := filepath.Join(gamePath, "addon.lua")
	if err := os.WriteFile(filePath, []byte("addon data"), 0o600); err != nil {
		t.Fatal(err)
	}

	files, err := listSavedVariablesFiles(gamePath)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 || files[0] != "addon.lua" {
		t.Fatalf("tracked paths = %v, want [addon.lua]", files)
	}
	size, oversized, err := savedVariablesFileSizes(gamePath, 100)
	if err != nil {
		t.Fatal(err)
	}
	if size != int64(len("addon data")) || len(oversized) != 0 {
		t.Fatalf("size/oversized = %d/%v, want %d/none", size, oversized, len("addon data"))
	}

	repoPath := t.TempDir()
	if err := copySavedVariablesFiles(gamePath, repoPath, 100); err != nil {
		t.Fatal(err)
	}
	copied, err := os.ReadFile(filepath.Join(repoPath, "addon.lua"))
	if err != nil || string(copied) != "addon data" {
		t.Fatalf("copied repository file = %q, err = %v", copied, err)
	}
}

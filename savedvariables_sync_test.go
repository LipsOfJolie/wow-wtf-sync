package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-git/go-git/v5"
	gitconfig "github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
)

const syncTestFile = "Realm/SavedVariables/addon.lua"

func TestAutomaticSavedVariablesAction(t *testing.T) {
	tests := []struct {
		name        string
		baseExists  bool
		baseBody    string
		localExists bool
		localBody   string
		localTime   time.Time
		cloudBody   string
		cloudTime   time.Time
		want        automaticSyncAction
	}{
		{
			name:        "matching hashes need no sync",
			baseExists:  true,
			baseBody:    "same",
			localExists: true,
			localBody:   "same",
			localTime:   time.Unix(1_600_000_000, 0),
			cloudBody:   "same",
			cloudTime:   time.Unix(1_700_000_000, 0),
			want:        autoSyncAlreadySynced,
		},
		{
			name:        "newer local file is pushed",
			baseExists:  true,
			baseBody:    "cloud",
			localExists: true,
			localBody:   "local",
			localTime:   time.Unix(1_700_000_001, 0),
			cloudBody:   "cloud",
			cloudTime:   time.Unix(1_700_000_000, 0),
			want:        autoSyncPush,
		},
		{
			name:        "newer cloud file is pulled",
			baseExists:  true,
			baseBody:    "base",
			localExists: true,
			localBody:   "local",
			localTime:   time.Unix(1_699_999_999, 0),
			cloudBody:   "cloud",
			cloudTime:   time.Unix(1_700_000_000, 0),
			want:        autoSyncPull,
		},
		{
			name:        "different hashes with equal timestamps prefer local file",
			baseExists:  true,
			baseBody:    "base",
			localExists: true,
			localBody:   "local",
			localTime:   time.Unix(1_700_000_000, 0),
			cloudBody:   "cloud",
			cloudTime:   time.Unix(1_700_000_000, 0),
			want:        autoSyncPush,
		},
		{
			name:        "file added locally is pushed",
			baseExists:  false,
			localExists: true,
			localBody:   "new local file",
			localTime:   time.Unix(1_700_000_001, 0),
			cloudBody:   "",
			cloudTime:   time.Time{},
			want:        autoSyncPush,
		},
		{
			name:        "file added in cloud is pulled",
			baseExists:  false,
			localExists: false,
			cloudBody:   "new cloud file",
			cloudTime:   time.Unix(1_700_000_000, 0),
			want:        autoSyncPull,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repoPath := t.TempDir()
			cloudTime := tt.cloudTime
			if cloudTime.IsZero() {
				cloudTime = time.Unix(1_700_000_000, 0)
			}
			basePath := "README.md"
			if tt.baseExists {
				basePath = syncTestFile
			}
			repo := initSavedVariablesRepository(t, repoPath, basePath, tt.baseBody, time.Unix(1_700_000_000, 0))
			baseHead, err := repo.Head()
			if err != nil {
				t.Fatal(err)
			}
			cloudPath := syncTestFile
			if !tt.baseExists && tt.name == "file added locally is pushed" {
				cloudPath = "README.md"
			}
			if cloudPath != basePath || tt.cloudBody != tt.baseBody {
				writeSyncTestFile(t, repoPath, cloudPath, tt.cloudBody, cloudTime)
				commitSyncTestFile(t, repo, repoPath, cloudPath, tt.cloudBody, cloudTime)
				updateRemoteTrackingRef(t, repo)
				resetLocalRepositoryTo(t, repo, baseHead.Hash())
			}
			gamePath := t.TempDir()
			if tt.localExists {
				writeSyncTestFile(t, gamePath, syncTestFile, tt.localBody, tt.localTime)
			}

			action, err := automaticSavedVariablesAction(gamePath, repoPath, 80*1024*1024)
			if err != nil {
				t.Fatal(err)
			}
			if action != tt.want {
				t.Errorf("automaticSavedVariablesAction() = %q, want %q", action, tt.want)
			}
		})
	}
}

func TestAutomaticSavedVariablesActionUsesLastCommitForEachFile(t *testing.T) {
	repoPath := t.TempDir()
	base := time.Unix(1_700_000_000, 0)
	repo := initSavedVariablesRepository(t, repoPath, syncTestFile, "initial", base)
	baseHead, err := repo.Head()
	if err != nil {
		t.Fatal(err)
	}

	unrelatedPath := "Realm/SavedVariables/other.lua"
	writeSyncTestFile(t, repoPath, syncTestFile, "updated", base.Add(2*time.Second))
	commitSyncTestFile(t, repo, repoPath, syncTestFile, "updated", base.Add(2*time.Second))
	writeSyncTestFile(t, repoPath, unrelatedPath, "later unrelated change", base.Add(10*time.Second))
	commitSyncTestFile(t, repo, repoPath, unrelatedPath, "later unrelated change", base.Add(10*time.Second))
	updateRemoteTrackingRef(t, repo)
	resetLocalRepositoryTo(t, repo, baseHead.Hash())

	gamePath := t.TempDir()
	writeSyncTestFile(t, gamePath, syncTestFile, "local version", base.Add(3*time.Second))
	writeSyncTestFile(t, gamePath, unrelatedPath, "later unrelated change", base.Add(10*time.Second))
	action, err := automaticSavedVariablesAction(gamePath, repoPath, 80*1024*1024)
	if err != nil {
		t.Fatal(err)
	}
	if action != autoSyncPush {
		t.Fatalf("action = %q, want %q (timestamp from the file's last commit)", action, autoSyncPush)
	}
}

func TestAutomaticSavedVariablesActionWhenSelectedFolderIsSavedVariables(t *testing.T) {
	repoPath := t.TempDir()
	base := time.Unix(1_700_000_000, 0)
	initSavedVariablesRepository(t, repoPath, "addon.lua", "cloud data", base)

	gamePath := filepath.Join(t.TempDir(), "SavedVariables")
	writeSyncTestFile(t, gamePath, "addon.lua", "new local data", base.Add(time.Second))
	action, err := automaticSavedVariablesAction(gamePath, repoPath, 80*1024*1024)
	if err != nil {
		t.Fatal(err)
	}
	if action != autoSyncPush {
		t.Fatalf("action = %q, want %q when selecting SavedVariables directly", action, autoSyncPush)
	}
}

func TestPullCopiesToSelectedSavedVariablesFolder(t *testing.T) {
	repoPath := t.TempDir()
	initSavedVariablesRepository(t, repoPath, "addon.lua", "cloud data", time.Unix(1_700_000_000, 0))
	gamePath := filepath.Join(t.TempDir(), "SavedVariables")
	if err := os.MkdirAll(gamePath, 0o700); err != nil {
		t.Fatal(err)
	}

	if err := copyTrackedSavedVariables(repoPath, gamePath, 80*1024*1024); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(gamePath, "addon.lua"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "cloud data" {
		t.Fatalf("restored file = %q, want cloud contents", got)
	}
}

func TestAutomaticSavedVariablesActionRequiresManualChoiceForMixedDirections(t *testing.T) {
	repoPath := t.TempDir()
	base := time.Unix(1_700_000_000, 0)
	firstPath := syncTestFile
	secondPath := "Realm/SavedVariables/second.lua"
	repo := initSavedVariablesRepository(t, repoPath, firstPath, "cloud first", base)
	baseHead, err := repo.Head()
	if err != nil {
		t.Fatal(err)
	}
	writeSyncTestFile(t, repoPath, secondPath, "cloud second", base.Add(time.Second))
	commitSyncTestFile(t, repo, repoPath, secondPath, "cloud second", base.Add(time.Second))
	updateRemoteTrackingRef(t, repo)
	resetLocalRepositoryTo(t, repo, baseHead.Hash())

	gamePath := t.TempDir()
	writeSyncTestFile(t, gamePath, firstPath, "local first", base.Add(2*time.Second))
	writeSyncTestFile(t, gamePath, secondPath, "local second", base)
	action, err := automaticSavedVariablesAction(gamePath, repoPath, 80*1024*1024)
	if err != nil {
		t.Fatal(err)
	}
	if action != autoSyncConflict {
		t.Fatalf("action = %q, want manual-choice conflict", action)
	}
}

func TestAutomaticSavedVariablesActionDetectsLocalAndCloudDeletions(t *testing.T) {
	tests := []struct {
		name        string
		deleteLocal bool
		want        automaticSyncAction
	}{
		{name: "local deletion is pushed", deleteLocal: true, want: autoSyncPush},
		{name: "cloud deletion is pulled", want: autoSyncPull},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repoPath := t.TempDir()
			gamePath := t.TempDir()
			base := time.Unix(1_700_000_000, 0)
			repo := initSavedVariablesRepository(t, repoPath, syncTestFile, "existing", base)
			baseHead, err := repo.Head()
			if err != nil {
				t.Fatal(err)
			}
			if !tt.deleteLocal {
				writeSyncTestFile(t, gamePath, syncTestFile, "existing", base)
				deleteSyncTestFileAndCommit(t, repo, syncTestFile, base.Add(time.Second))
				updateRemoteTrackingRef(t, repo)
				resetLocalRepositoryTo(t, repo, baseHead.Hash())
			}

			action, err := automaticSavedVariablesAction(gamePath, repoPath, 80*1024*1024)
			if err != nil {
				t.Fatal(err)
			}
			if action != tt.want {
				t.Fatalf("automaticSavedVariablesAction() = %q, want %q", action, tt.want)
			}
		})
	}
}

func initSavedVariablesRepository(t *testing.T, path, filePath, contents string, committedAt time.Time) *git.Repository {
	t.Helper()
	repo, err := git.PlainInit(path, false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repo.CreateRemote(&gitconfig.RemoteConfig{Name: "origin", URLs: []string{"https://github.com/test/repo.git"}}); err != nil {
		t.Fatal(err)
	}
	writeSyncTestFile(t, path, filePath, contents, committedAt)
	commitSyncTestFile(t, repo, path, filePath, contents, committedAt)
	head, err := repo.Head()
	if err != nil {
		t.Fatal(err)
	}
	updateRemoteTrackingRef(t, repo)
	repoConfig, err := repo.Config()
	if err != nil {
		t.Fatal(err)
	}
	branchName := head.Name().Short()
	repoConfig.Branches[branchName] = &gitconfig.Branch{
		Name:   branchName,
		Remote: "origin",
		Merge:  plumbing.NewBranchReferenceName(branchName),
	}
	if err := repo.SetConfig(repoConfig); err != nil {
		t.Fatal(err)
	}
	return repo
}

func updateRemoteTrackingRef(t *testing.T, repo *git.Repository) {
	t.Helper()
	head, err := repo.Head()
	if err != nil {
		t.Fatal(err)
	}
	branchName := head.Name().Short()
	ref := plumbing.NewHashReference(plumbing.NewRemoteReferenceName("origin", branchName), head.Hash())
	if err := repo.Storer.SetReference(ref); err != nil {
		t.Fatal(err)
	}
}

func resetLocalRepositoryTo(t *testing.T, repo *git.Repository, hash plumbing.Hash) {
	t.Helper()
	worktree, err := repo.Worktree()
	if err != nil {
		t.Fatal(err)
	}
	if err := worktree.Reset(&git.ResetOptions{Mode: git.HardReset, Commit: hash}); err != nil {
		t.Fatal(err)
	}
}

func deleteSyncTestFileAndCommit(t *testing.T, repo *git.Repository, filePath string, committedAt time.Time) {
	t.Helper()
	worktree, err := repo.Worktree()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := worktree.Remove(filePath); err != nil {
		t.Fatal(err)
	}
	if _, err := worktree.Commit("test delete", &git.CommitOptions{
		Author: &object.Signature{Name: "Test", Email: "test@example.com", When: committedAt},
	}); err != nil {
		t.Fatal(err)
	}
}

func commitSyncTestFile(t *testing.T, repo *git.Repository, root, filePath, contents string, committedAt time.Time) {
	t.Helper()
	worktree, err := repo.Worktree()
	if err != nil {
		t.Fatal(err)
	}
	fullPath := filepath.Join(root, filepath.FromSlash(filePath))
	if err := os.MkdirAll(filepath.Dir(fullPath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(fullPath, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := worktree.Add(filePath); err != nil {
		t.Fatal(err)
	}
	_, err = worktree.Commit("test update", &git.CommitOptions{
		Author:    &object.Signature{Name: "Test", Email: "test@example.com", When: committedAt},
		Committer: &object.Signature{Name: "Test", Email: "test@example.com", When: committedAt},
	})
	if err != nil {
		t.Fatal(err)
	}
}

func writeSyncTestFile(t *testing.T, root, filePath, contents string, modifiedAt time.Time) {
	t.Helper()
	fullPath := filepath.Join(root, filepath.FromSlash(filePath))
	if err := os.MkdirAll(filepath.Dir(fullPath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(fullPath, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
	if !modifiedAt.IsZero() {
		if err := os.Chtimes(fullPath, modifiedAt, modifiedAt); err != nil {
			t.Fatal(err)
		}
	}
}

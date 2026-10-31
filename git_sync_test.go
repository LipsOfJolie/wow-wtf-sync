package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
)

func TestClassifyRepoStatus(t *testing.T) {
	tests := []struct {
		name            string
		remoteAhead     bool
		localAhead      bool
		hasLocalChanges bool
		want            string
	}{
		{
			name:        "cloud is newer",
			remoteAhead: true,
			want:        RepoStatusCloudNewer,
		},
		{
			name:       "local branch is newer",
			localAhead: true,
			want:       RepoStatusLocalNewer,
		},
		{
			name:            "local files have uncommitted changes",
			hasLocalChanges: true,
			want:            RepoStatusLocalNewer,
		},
		{
			name:        "both branches have changes",
			remoteAhead: true,
			localAhead:  true,
			want:        RepoStatusConflict,
		},
		{
			name:            "cloud and local files have changes",
			remoteAhead:     true,
			hasLocalChanges: true,
			want:            RepoStatusConflict,
		},
		{
			name: "both are up to date",
			want: RepoStatusSynced,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := classifyRepoStatus(tt.remoteAhead, tt.localAhead, tt.hasLocalChanges)
			if got != tt.want {
				t.Errorf("classifyRepoStatus() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRetryRepositoryStatusRetriesAndReturnsSuccess(t *testing.T) {
	attempts := 0
	var waits []time.Duration
	status, err := retryRepositoryStatus(3, 5*time.Second, func(duration time.Duration) {
		waits = append(waits, duration)
	}, func() (string, error) {
		attempts++
		if attempts < 3 {
			return "", errors.New("offline")
		}
		return RepoStatusSynced, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if status != RepoStatusSynced || attempts != 3 {
		t.Errorf("result = (%q, %d attempts), want (%q, 3 attempts)", status, attempts, RepoStatusSynced)
	}
	if len(waits) != 2 || waits[0] != 5*time.Second || waits[1] != 5*time.Second {
		t.Errorf("retry waits = %v, want two 5-second waits", waits)
	}
}

func TestGitPushRestagesSynchronizedFileInsteadOfRejectingIt(t *testing.T) {
	repoPath := t.TempDir()
	repo, err := git.PlainInit(repoPath, false)
	if err != nil {
		t.Fatal(err)
	}
	commitRepositoryFile(t, repo, repoPath, "Account/addon.lua", "initial")
	if err := os.WriteFile(filepath.Join(repoPath, "Account", "addon.lua"), []byte("staged version"), 0o600); err != nil {
		t.Fatal(err)
	}
	worktree, err := repo.Worktree()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := worktree.Add("Account/addon.lua"); err != nil {
		t.Fatal(err)
	}

	err = GitPush(repoPath, "", "test push", []string{"Account/addon.lua"})
	if err == nil || strings.Contains(err.Error(), "staged changes") {
		t.Fatalf("GitPush error = %v, want only an eventual remote error, not staged-change rejection", err)
	}
	head, err := repo.Head()
	if err != nil {
		t.Fatal(err)
	}
	commit, err := repo.CommitObject(head.Hash())
	if err != nil {
		t.Fatal(err)
	}
	tree, err := commit.Tree()
	if err != nil {
		t.Fatal(err)
	}
	file, err := tree.File("Account/addon.lua")
	if err != nil {
		t.Fatal(err)
	}
	contents, err := file.Contents()
	if err != nil {
		t.Fatal(err)
	}
	if contents != "staged version" {
		t.Fatalf("committed file = %q, want staged synchronized contents", contents)
	}
}

func TestGitPushDoesNotCommitFilesOnlyPresentInLocalRepository(t *testing.T) {
	repoPath := t.TempDir()
	repo, err := git.PlainInit(repoPath, false)
	if err != nil {
		t.Fatal(err)
	}
	commitRepositoryFile(t, repo, repoPath, "Account/current.lua", "previous")
	if err := os.WriteFile(filepath.Join(repoPath, "Presentation.odp"), []byte("staged unrelated file"), 0o600); err != nil {
		t.Fatal(err)
	}
	worktree, err := repo.Worktree()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := worktree.Add("Presentation.odp"); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repoPath, "Account", "current.lua"), []byte("current selected data"), 0o600); err != nil {
		t.Fatal(err)
	}

	err = GitPush(repoPath, "", "test push", []string{"Account/current.lua"})
	if err == nil {
		t.Fatal("GitPush unexpectedly succeeded without a configured remote")
	}
	head, err := repo.Head()
	if err != nil {
		t.Fatal(err)
	}
	commit, err := repo.CommitObject(head.Hash())
	if err != nil {
		t.Fatal(err)
	}
	tree, err := commit.Tree()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tree.File("Presentation.odp"); err == nil {
		t.Fatal("file present only in local repository was committed")
	}
	file, err := tree.File("Account/current.lua")
	if err != nil {
		t.Fatal(err)
	}
	contents, err := file.Contents()
	if err != nil {
		t.Fatal(err)
	}
	if contents != "current selected data" {
		t.Fatalf("selected file = %q, want current selected data", contents)
	}
	if _, err := os.Stat(filepath.Join(repoPath, "Presentation.odp")); err != nil {
		t.Fatalf("unstaged unrelated file should remain in local repository: %v", err)
	}
}

func TestDivergedBranchStatusPreservesBothVersions(t *testing.T) {
	localPath := t.TempDir()
	localRepo, err := git.PlainInit(localPath, false)
	if err != nil {
		t.Fatal(err)
	}
	const savedFile = "Realm/SavedVariables/addon.lua"
	commitRepositoryFile(t, localRepo, localPath, savedFile, "base")
	baseHead, err := localRepo.Head()
	if err != nil {
		t.Fatal(err)
	}
	worktree, err := localRepo.Worktree()
	if err != nil {
		t.Fatal(err)
	}
	if err := worktree.Checkout(&git.CheckoutOptions{Hash: baseHead.Hash()}); err != nil {
		t.Fatal(err)
	}
	commitRepositoryFile(t, localRepo, localPath, savedFile, "remote version")
	remoteHead, err := localRepo.Head()
	if err != nil {
		t.Fatal(err)
	}
	if err := worktree.Checkout(&git.CheckoutOptions{Hash: baseHead.Hash()}); err != nil {
		t.Fatal(err)
	}
	commitRepositoryFile(t, localRepo, localPath, savedFile, "local version")
	localHead, err := localRepo.Head()
	if err != nil {
		t.Fatal(err)
	}
	localCommit, err := localRepo.CommitObject(localHead.Hash())
	if err != nil {
		t.Fatal(err)
	}
	remoteCommit, err := localRepo.CommitObject(remoteHead.Hash())
	if err != nil {
		t.Fatal(err)
	}
	localIsAncestor, err := localCommit.IsAncestor(remoteCommit)
	if err != nil {
		t.Fatal(err)
	}
	remoteIsAncestor, err := remoteCommit.IsAncestor(localCommit)
	if err != nil {
		t.Fatal(err)
	}
	if localIsAncestor || remoteIsAncestor {
		t.Fatal("test setup did not create diverged histories")
	}
	if status := classifyRepoStatus(true, true, false); status != RepoStatusConflict {
		t.Fatalf("diverged versions classified as %q, want conflict", status)
	}

	for _, version := range []struct {
		hash    plumbing.Hash
		content string
	}{
		{localHead.Hash(), "local version"},
		{remoteHead.Hash(), "remote version"},
	} {
		commit, err := localRepo.CommitObject(version.hash)
		if err != nil {
			t.Fatalf("diverged commit %s was lost: %v", version.content, err)
		}
		tree, err := commit.Tree()
		if err != nil {
			t.Fatal(err)
		}
		file, err := tree.File(savedFile)
		if err != nil {
			t.Fatal(err)
		}
		content, err := file.Contents()
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(content, version.content) {
			t.Errorf("commit %s contains %q, want %q", version.hash.String(), content, version.content)
		}
	}
}

func commitRepositoryFile(t *testing.T, repo *git.Repository, root, relativePath, content string) {
	t.Helper()
	worktree, err := repo.Worktree()
	if err != nil {
		t.Fatal(err)
	}
	fullPath := filepath.Join(root, filepath.FromSlash(relativePath))
	if err := os.MkdirAll(filepath.Dir(fullPath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(fullPath, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := worktree.Add(relativePath); err != nil {
		t.Fatal(err)
	}
	if _, err := worktree.Commit("test update", &git.CommitOptions{
		Author: &object.Signature{Name: "Test", Email: "test@example.com", When: time.Now()},
	}); err != nil {
		t.Fatal(err)
	}
}

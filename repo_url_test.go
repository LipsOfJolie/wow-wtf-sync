package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/go-git/go-git/v5"
	gitconfig "github.com/go-git/go-git/v5/config"
)

func TestValidateGitHubRepoURL(t *testing.T) {
	tests := []struct {
		input string
		valid bool
	}{
		{"https://github.com/owner/repository", true},
		{"https://github.com/owner/repository.git", true},
		{"HTTPS://GITHUB.COM/owner/repository", true},
		{"https://github.com.evil.example/owner/repository", false},
		{"https://evil.example/github.com/owner/repository", false},
		{"http://github.com/owner/repository", false},
		{"https://user:password@github.com/owner/repository", false},
		{"https://github.com:444/owner/repository", false},
		{"https://github.com:/owner/repository", false},
		{"https://github.com/owner/repository?redirect=evil", false},
		{"https://github.com/owner/repository?", false},
		{"https://github.com/owner/repository#", false},
		{"https://github.com/owner%2frepository", false},
		{"https://github.com/owner/repository/extra", false},
		{"https://github.com/../repository", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := validateGitHubRepoURL(tt.input)
			if (err == nil) != tt.valid {
				t.Fatalf("validateGitHubRepoURL(%q) error = %v, valid = %t", tt.input, err, tt.valid)
			}
			if tt.valid && got != "https://github.com/owner/repository.git" {
				t.Errorf("normalized URL = %q", got)
			}
		})
	}
}

func TestInvalidRepositoryURLDoesNotSendTokenToServer(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		requests.Add(1)
	}))
	defer server.Close()

	if CheckRepoValid(server.URL+"/owner/repository", "sensitive-test-token") {
		t.Fatal("an HTTP URL should not be accepted as a GitHub repository")
	}
	if got := requests.Load(); got != 0 {
		t.Fatalf("invalid repository URL triggered %d HTTP requests", got)
	}
}

func TestRepositoryURLChangePreservesExistingCopy(t *testing.T) {
	parent := t.TempDir()
	localPath := filepath.Join(parent, LocalRepoFolder)
	repo, err := git.PlainInit(localPath, false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repo.CreateRemote(&gitconfig.RemoteConfig{
		Name: "origin",
		URLs: []string{"https://github.com/old-owner/old-repo.git"},
	}); err != nil {
		t.Fatal(err)
	}
	dataPath := filepath.Join(localPath, "keep.txt")
	if err := os.WriteFile(dataPath, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}

	_, err = GetRepository(localPath, "https://github.com/new-owner/new-repo.git", "sensitive-test-token")
	if err == nil {
		t.Fatal("repository URL change should be reported")
	}
	if strings.Contains(err.Error(), "sensitive-test-token") {
		t.Fatal("repository error exposed the token")
	}
	if _, err := os.Stat(localPath); !os.IsNotExist(err) {
		t.Fatalf("old working copy remains at active path: %v", err)
	}
	entries, err := os.ReadDir(parent)
	if err != nil {
		t.Fatal(err)
	}
	var preservedPath string
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), LocalRepoFolder+".repository-changed.") {
			preservedPath = filepath.Join(parent, entry.Name())
		}
	}
	if preservedPath == "" {
		t.Fatal("old working copy was not preserved under a backup name")
	}
	if data, err := os.ReadFile(filepath.Join(preservedPath, "keep.txt")); err != nil || string(data) != "keep" {
		t.Fatalf("preserved working copy contents = %q, err = %v", data, err)
	}
}

func TestDamagedRepositoryIsPreserved(t *testing.T) {
	parent := t.TempDir()
	localPath := filepath.Join(parent, LocalRepoFolder)
	if err := os.MkdirAll(filepath.Join(localPath, ".git"), 0o700); err != nil {
		t.Fatal(err)
	}
	keepPath := filepath.Join(localPath, "keep.txt")
	if err := os.WriteFile(keepPath, []byte("local data"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(localPath, ".git", "config"), []byte("invalid git config"), 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := GetRepository(localPath, "https://github.com/owner/repository.git", "")
	if err == nil {
		t.Fatal("damaged repository should not be used")
	}
	entries, err := os.ReadDir(parent)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), LocalRepoFolder+".damaged.") {
			if data, readErr := os.ReadFile(filepath.Join(parent, entry.Name(), "keep.txt")); readErr == nil && string(data) == "local data" {
				return
			}
		}
	}
	t.Fatal("damaged working copy data was not preserved")
}

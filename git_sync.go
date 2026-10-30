package main

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	githttp "github.com/go-git/go-git/v5/plumbing/transport/http"
)

const (
	RepoStatusCloudNewer = "cloud_newer"
	RepoStatusLocalNewer = "local_newer"
	RepoStatusConflict   = "conflict"
	RepoStatusSynced     = "synced"
)

// Проверка валидности GitHub токена через API
func CheckTokenValid(token string) bool {
	if token == "" {
		return false
	}
	req, err := http.NewRequest("GET", "https://api.github.com/user", nil)
	if err != nil {
		return false
	}
	req.Header.Set("Authorization", "token "+token)
	client := &http.Client{Timeout: 4 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == 200
}

// Проверка существования репозитория через GitHub API
func CheckRepoValid(repoURL string, token string) bool {
	cleanURL := strings.TrimSuffix(strings.TrimSpace(repoURL), "/")
	cleanURL = strings.TrimSuffix(cleanURL, ".git")
	parts := strings.Split(cleanURL, "github.com/")
	if len(parts) < 2 {
		return false
	}

	apiURL := "https://api.github.com/repos/" + parts[1]
	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return false
	}
	if token != "" {
		req.Header.Set("Authorization", "token "+token)
	}
	client := &http.Client{Timeout: 4 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == 200
}

// Подсчет размера папки в байтах
func GetFolderSize(path string) (int64, error) {
	var size int64
	err := filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			size += info.Size()
		}
		return nil
	})
	return size, err
}

// Получение или клонирование репозитория
func GetRepository(localPath string, repoURL string, token string) (*git.Repository, error) {
	dotGitPath := filepath.Join(localPath, ".git")

	var auth *githttp.BasicAuth
	if token != "" {
		auth = &githttp.BasicAuth{
			Username: "x-access-token",
			Password: token,
		}
	}

	if _, err := os.Stat(dotGitPath); os.IsNotExist(err) {
		_ = os.RemoveAll(localPath)

		return git.PlainClone(localPath, false, &git.CloneOptions{
			URL:  repoURL,
			Auth: auth,
		})
	}

	return git.PlainOpen(localPath)
}

// Git Pull (скачивание изменений)
func GitPull(localPath string, token string) error {
	repo, err := git.PlainOpen(localPath)
	if err != nil {
		return err
	}

	worktree, err := repo.Worktree()
	if err != nil {
		return err
	}

	var auth *githttp.BasicAuth
	if token != "" {
		auth = &githttp.BasicAuth{
			Username: "x-access-token",
			Password: token,
		}
	}

	err = worktree.Pull(&git.PullOptions{
		RemoteName: "origin",
		Auth:       auth,
		Force:      true,
	})

	if err == git.NoErrAlreadyUpToDate {
		return nil
	}
	return err
}

// Git Push (отправка изменений)
func GitPush(localPath string, token string, message string) error {
	repo, err := git.PlainOpen(localPath)
	if err != nil {
		return err
	}

	worktree, err := repo.Worktree()
	if err != nil {
		return err
	}

	_, err = worktree.Add(".")
	if err != nil {
		return err
	}

	_, err = worktree.Commit(message, &git.CommitOptions{
		Author: &object.Signature{
			Name:  "WoW Sync Launcher",
			Email: "launcher@wow.local",
			When:  time.Now(),
		},
	})
	if err != nil {
		status, statusErr := worktree.Status()
		if statusErr == nil && status.IsClean() {
			return nil
		}
	}

	var auth *githttp.BasicAuth
	if token != "" {
		auth = &githttp.BasicAuth{
			Username: "x-access-token",
			Password: token,
		}
	}

	err = repo.Push(&git.PushOptions{
		Auth: auth,
	})

	if err == git.NoErrAlreadyUpToDate {
		return nil
	}
	return err
}

// CheckRepoStatus compares the local working tree and branch with the remote branch.
func CheckRepoStatus(localPath string, token string) (string, error) {
	repo, err := git.PlainOpen(localPath)
	if err != nil {
		return "", err
	}

	head, err := repo.Head()
	if err != nil {
		return "", err
	}

	repoConfig, err := repo.Config()
	if err != nil {
		return "", err
	}

	branchName := head.Name().Short()
	branch, ok := repoConfig.Branches[branchName]
	if !ok || branch.Remote == "" || branch.Merge == "" {
		return "", fmt.Errorf("ветка %q не связана с удаленной веткой", branchName)
	}

	var auth *githttp.BasicAuth
	if token != "" {
		auth = &githttp.BasicAuth{
			Username: "x-access-token",
			Password: token,
		}
	}

	err = repo.Fetch(&git.FetchOptions{
		RemoteName: branch.Remote,
		Auth:       auth,
		Force:      true,
	})
	if err != nil && err != git.NoErrAlreadyUpToDate {
		return "", fmt.Errorf("не удалось получить статус удаленного репозитория: %w", err)
	}

	remoteRefName := plumbing.NewRemoteReferenceName(branch.Remote, branch.Merge.Short())
	remoteRef, err := repo.Reference(remoteRefName, true)
	if err != nil {
		return "", fmt.Errorf("не удалось найти удаленную ветку %q: %w", remoteRefName, err)
	}

	localCommit, err := repo.CommitObject(head.Hash())
	if err != nil {
		return "", err
	}
	remoteCommit, err := repo.CommitObject(remoteRef.Hash())
	if err != nil {
		return "", err
	}

	remoteAhead := false
	localAhead := false
	if localCommit.Hash != remoteCommit.Hash {
		localIsAncestor, err := localCommit.IsAncestor(remoteCommit)
		if err != nil {
			return "", err
		}
		remoteIsAncestor, err := remoteCommit.IsAncestor(localCommit)
		if err != nil {
			return "", err
		}
		remoteAhead = localIsAncestor
		localAhead = remoteIsAncestor
		if !remoteAhead && !localAhead {
			remoteAhead = true
			localAhead = true
		}
	}

	worktree, err := repo.Worktree()
	if err != nil {
		return "", err
	}

	status, err := worktree.Status()
	if err != nil {
		return "", err
	}

	return classifyRepoStatus(remoteAhead, localAhead, !status.IsClean()), nil
}

func classifyRepoStatus(remoteAhead, localAhead, hasLocalChanges bool) string {
	if remoteAhead && (localAhead || hasLocalChanges) {
		return RepoStatusConflict
	}
	if remoteAhead {
		return RepoStatusCloudNewer
	}
	if localAhead || hasLocalChanges {
		return RepoStatusLocalNewer
	}
	return RepoStatusSynced
}

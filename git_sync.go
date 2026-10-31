package main

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	gitclient "github.com/go-git/go-git/v5/plumbing/transport/client"
	githttp "github.com/go-git/go-git/v5/plumbing/transport/http"
)

const gitNetworkTimeout = 60 * time.Second
const gitStartupAttempts = 3
const gitStartupRetryInterval = 5 * time.Second

func init() {
	gitclient.InstallProtocol("https", githttp.NewClient(&http.Client{Timeout: gitNetworkTimeout}))
}

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
	closeErr := resp.Body.Close()
	return resp.StatusCode == http.StatusOK && closeErr == nil
}

// Проверка существования репозитория через GitHub API
func CheckRepoValid(repoURL string, token string) bool {
	cleanURL, err := validateGitHubRepoURL(repoURL)
	if err != nil {
		return false
	}
	parts := strings.Split(strings.TrimSuffix(strings.TrimPrefix(cleanURL, "https://github.com/"), ".git"), "/")
	apiURL := "https://api.github.com/repos/" + url.PathEscape(parts[0]) + "/" + url.PathEscape(parts[1])
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
	closeErr := resp.Body.Close()
	return resp.StatusCode == http.StatusOK && closeErr == nil
}

// Получение или клонирование репозитория
func GetRepository(localPath string, repoURL string, token string) (*git.Repository, error) {
	repoURL, err := validateGitHubRepoURL(repoURL)
	if err != nil {
		return nil, err
	}
	if err := rejectUnsafePathComponents(localPath); err != nil {
		return nil, err
	}

	dotGitPath := filepath.Join(localPath, ".git")
	if err := rejectUnsafePathComponents(dotGitPath); err != nil {
		return nil, fmt.Errorf("local repository metadata path is unsafe: %w", err)
	}

	var auth *githttp.BasicAuth
	if token != "" {
		auth = &githttp.BasicAuth{
			Username: "x-access-token",
			Password: token,
		}
	}

	if _, err := os.Stat(dotGitPath); errors.Is(err, os.ErrNotExist) {
		if _, statErr := os.Stat(localPath); statErr == nil {
			backupPath, preserveErr := preserveLocalRepository(localPath, "uninitialized")
			if preserveErr != nil {
				return nil, preserveErr
			}
			return nil, &repositoryPreservedError{path: backupPath}
		} else if !errors.Is(statErr, os.ErrNotExist) {
			return nil, statErr
		}
		return git.PlainClone(localPath, false, &git.CloneOptions{
			URL:  repoURL,
			Auth: auth,
		})
	} else if err != nil {
		return nil, err
	}

	repo, err := git.PlainOpen(localPath)
	if err != nil {
		backupPath, preserveErr := preserveLocalRepository(localPath, "damaged")
		if preserveErr != nil {
			return nil, fmt.Errorf("local repository is damaged (%v); could not preserve it: %w", err, preserveErr)
		}
		return nil, &repositoryPreservedError{path: backupPath}
	}
	if err := repositoryOriginMatches(repo, repoURL); err != nil {
		backupPath, preserveErr := preserveLocalRepository(localPath, "repository-changed")
		if preserveErr != nil {
			return nil, fmt.Errorf("%w; preservation failed: %v", err, preserveErr)
		}
		return nil, &repositoryPreservedError{path: backupPath}
	}
	return repo, nil
}

func repositoryOriginMatches(repo *git.Repository, configuredURL string) error {
	remote, err := repo.Remote("origin")
	if err != nil {
		return fmt.Errorf("local repository has no origin remote; refusing to use it")
	}
	remoteConfig := remote.Config()
	if len(remoteConfig.URLs) != 1 {
		return fmt.Errorf("local repository has an ambiguous origin; refusing to use it")
	}
	originURL, err := validateGitHubRepoURL(remoteConfig.URLs[0])
	if err != nil || originURL != configuredURL {
		return fmt.Errorf("local repository origin does not match the configured repository")
	}
	return nil
}

type repositoryPreservedError struct {
	path string
}

func (err *repositoryPreservedError) Error() string {
	return fmt.Sprintf("local repository data was preserved at %q; retry to use the configured repository", err.path)
}

func preserveLocalRepository(localPath, reason string) (string, error) {
	parent := filepath.Dir(localPath)
	base := filepath.Base(localPath)
	for attempt := 0; attempt < 100; attempt++ {
		suffix := time.Now().Format("20060102-150405.000000000")
		if attempt > 0 {
			suffix += fmt.Sprintf("-%d", attempt)
		}
		backupPath := filepath.Join(parent, base+"."+reason+"."+suffix)
		if _, err := os.Lstat(backupPath); err == nil {
			continue
		} else if !errors.Is(err, os.ErrNotExist) {
			return "", err
		}
		if err := os.Rename(localPath, backupPath); err != nil {
			return "", err
		}
		return backupPath, nil
	}
	return "", fmt.Errorf("could not find an unused backup name for local repository")
}

func getRepositoryStatusWithRetries(localPath, repoURL, token string, attempts int, interval time.Duration, wait func(time.Duration)) (string, error) {
	return retryRepositoryStatus(attempts, interval, wait, func() (string, error) {
		if _, err := GetRepository(localPath, repoURL, token); err != nil {
			return "", &repositoryStatusError{stage: "repository", err: err}
		}
		status, err := CheckRepoStatus(localPath, token)
		if err != nil {
			return "", &repositoryStatusError{stage: "status", err: err}
		}
		return status, nil
	})
}

type repositoryStatusError struct {
	stage string
	err   error
}

func (err *repositoryStatusError) Error() string {
	return err.err.Error()
}

func (err *repositoryStatusError) Unwrap() error {
	return err.err
}

func retryRepositoryStatus(attempts int, interval time.Duration, wait func(time.Duration), check func() (string, error)) (string, error) {
	var lastErr error
	for attempt := 0; attempt < attempts; attempt++ {
		status, err := check()
		if err == nil {
			return status, nil
		}
		lastErr = err
		var preservedErr *repositoryPreservedError
		if errors.As(err, &preservedErr) {
			break
		}
		if attempt+1 < attempts {
			wait(interval)
		}
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("попытки подключения к репозиторию не выполнялись")
	}
	return "", lastErr
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
	status, err := worktree.Status()
	if err != nil {
		return err
	}
	if !status.IsClean() {
		return errors.New("local Git working copy has uncommitted changes; Pull was cancelled")
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
	})

	if err == git.NoErrAlreadyUpToDate {
		return nil
	}
	return err
}

func ensureCleanWorktree(localPath string) error {
	repo, err := git.PlainOpen(localPath)
	if err != nil {
		return err
	}
	worktree, err := repo.Worktree()
	if err != nil {
		return err
	}
	status, err := worktree.Status()
	if err != nil {
		return err
	}
	if !status.IsClean() {
		return errors.New("local Git working copy has uncommitted changes")
	}
	return nil
}

// Git Push (отправка изменений)
func GitPush(localPath string, token string, message string, selectedPaths []string) error {
	repo, err := git.PlainOpen(localPath)
	if err != nil {
		return err
	}

	worktree, err := repo.Worktree()
	if err != nil {
		return err
	}

	trackedPaths, err := trackedRepositoryFiles(localPath, int64(MaxPushFileSizeMB)*1024*1024)
	if err != nil {
		return err
	}
	status, err := worktree.Status()
	if err != nil {
		return err
	}
	for path, fileStatus := range status {
		if fileStatus.Staging != git.Unmodified {
			if err := worktree.Reset(&git.ResetOptions{Mode: git.MixedReset, Files: []string{filepath.ToSlash(path)}}); err != nil {
				return fmt.Errorf("unstage local change at %q before synchronizing: %w", path, err)
			}
		}
	}

	pathsToStage := make(map[string]struct{}, len(selectedPaths)+len(trackedPaths))
	trackedPathSet := make(map[string]struct{}, len(trackedPaths))
	for _, path := range selectedPaths {
		pathsToStage[filepath.ToSlash(path)] = struct{}{}
	}
	for _, path := range trackedPaths {
		path = filepath.ToSlash(path)
		pathsToStage[path] = struct{}{}
		trackedPathSet[path] = struct{}{}
	}
	for path := range pathsToStage {
		fullPath, err := securePathUnder(localPath, filepath.FromSlash(path))
		if err != nil {
			return err
		}
		info, err := os.Lstat(fullPath)
		if errors.Is(err, os.ErrNotExist) {
			if _, wasTracked := trackedPathSet[path]; wasTracked {
				if _, err := worktree.Remove(path); err != nil {
					return fmt.Errorf("stage removal of %q: %w", path, err)
				}
			}
			continue
		}
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 || isReparsePoint(info) || !info.Mode().IsRegular() {
			return fmt.Errorf("selected-folder file is not regular: %q", path)
		}
		if info.Size() > int64(MaxPushFileSizeMB)*1024*1024 {
			continue
		}
		if _, err := worktree.Add(path); err != nil {
			return fmt.Errorf("stage selected-folder file %q: %w", path, err)
		}
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
		if statusErr != nil {
			return fmt.Errorf("не удалось проверить изменения после ошибки commit: %w", statusErr)
		}
		if !status.IsClean() {
			return err
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

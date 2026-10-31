package main

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
)

var errPullConfirmationRequired = errors.New("pull requires confirmation because selected folder differs")

type OversizedFile struct {
	Path string
	Size int64
}

type savedVariableVersion struct {
	hash     [32]byte
	modified time.Time
}

func automaticSavedVariablesAction(gamePath, repoPath string, maxFileSize int64) (automaticSyncAction, error) {
	gameFiles, err := savedVariableVersionsOnDisk(gamePath, maxFileSize)
	if err != nil {
		return "", err
	}
	repo, err := git.PlainOpen(repoPath)
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
	branch, ok := repoConfig.Branches[head.Name().Short()]
	if !ok || branch.Remote == "" || branch.Merge == "" {
		return "", fmt.Errorf("current Git branch is not tracking a remote branch")
	}
	remoteRef, err := repo.Reference(plumbing.NewRemoteReferenceName(branch.Remote, branch.Merge.Short()), true)
	if err != nil {
		return "", fmt.Errorf("find remote tracking branch: %w", err)
	}
	localCommit, err := repo.CommitObject(head.Hash())
	if err != nil {
		return "", err
	}
	cloudCommit, err := repo.CommitObject(remoteRef.Hash())
	if err != nil {
		return "", err
	}
	baseCommits, err := localCommit.MergeBase(cloudCommit)
	if err != nil {
		return "", err
	}
	if len(baseCommits) == 0 {
		return autoSyncConflict, nil
	}
	baseFiles, err := savedVariableVersionsInCommit(baseCommits[0], maxFileSize)
	if err != nil {
		return "", err
	}
	cloudFiles, err := savedVariableVersionsInCommit(cloudCommit, maxFileSize)
	if err != nil {
		return "", err
	}

	direction := automaticSyncAction("")
	paths := make(map[string]struct{}, len(gameFiles)+len(cloudFiles))
	for path := range gameFiles {
		paths[path] = struct{}{}
	}
	for path := range cloudFiles {
		paths[path] = struct{}{}
	}
	for path := range paths {
		local, existsLocally := gameFiles[path]
		cloud, existsInCloud := cloudFiles[path]
		if sameVersion(local, existsLocally, cloud, existsInCloud) {
			continue
		}

		base, existsInBase := baseFiles[path]
		localChanged := !sameVersion(local, existsLocally, base, existsInBase)
		cloudChanged := !sameVersion(cloud, existsInCloud, base, existsInBase)
		fileDirection := autoSyncConflict
		switch {
		case localChanged && !cloudChanged:
			fileDirection = autoSyncPush
		case !localChanged && cloudChanged:
			fileDirection = autoSyncPull
		case localChanged && cloudChanged && existsLocally && existsInCloud:
			cloudTime, err := lastCommitTimeForPath(repo, remoteRef.Hash(), path)
			if err != nil {
				return "", err
			}
			localTime := local.modified.Truncate(time.Second)
			cloudTime = cloudTime.Truncate(time.Second)
			if !localTime.Before(cloudTime) {
				fileDirection = autoSyncPush
			} else {
				fileDirection = autoSyncPull
			}
		}
		if fileDirection == autoSyncConflict || (direction != "" && direction != fileDirection) {
			return autoSyncConflict, nil
		}
		direction = fileDirection
	}
	if direction == "" {
		return autoSyncAlreadySynced, nil
	}
	return direction, nil
}

func sameVersion(first savedVariableVersion, firstExists bool, second savedVariableVersion, secondExists bool) bool {
	return firstExists == secondExists && (!firstExists || first.hash == second.hash)
}

func savedVariableVersionsOnDisk(root string, maxFileSize int64) (map[string]savedVariableVersion, error) {
	if err := rejectUnsafePathComponents(root); err != nil {
		return nil, err
	}
	files := make(map[string]savedVariableVersion)
	err := filepath.Walk(root, func(filePath string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if info.Mode()&os.ModeSymlink != 0 || isReparsePoint(info) {
			return fmt.Errorf("symbolic link is not allowed: %q", filePath)
		}
		relative, err := savedVariablesRelativePath(root, filePath)
		if err != nil {
			return err
		}
		if relative == ".git" && info.IsDir() {
			return filepath.SkipDir
		}
		if relative == "." || !info.Mode().IsRegular() {
			return nil
		}
		if isRepositoryMetadataPath(relative) {
			return nil
		}
		if maxFileSize > 0 && info.Size() > maxFileSize {
			return nil
		}
		file, err := os.Open(filePath)
		if err != nil {
			return err
		}
		hash := sha256.New()
		_, readErr := io.Copy(hash, file)
		closeErr := file.Close()
		if err := errors.Join(readErr, closeErr); err != nil {
			return err
		}
		var sum [32]byte
		copy(sum[:], hash.Sum(nil))
		files[relative] = savedVariableVersion{hash: sum, modified: info.ModTime()}
		return nil
	})
	return files, err
}

func savedVariableVersionsInCommit(commit *object.Commit, maxFileSize int64) (map[string]savedVariableVersion, error) {
	tree, err := commit.Tree()
	if err != nil {
		return nil, err
	}
	files := tree.Files()
	defer files.Close()

	versions := make(map[string]savedVariableVersion)
	for {
		file, err := files.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}
		if file.Name == "" {
			continue
		}
		if isRepositoryMetadataPath(file.Name) {
			continue
		}
		if !file.Mode.IsRegular() {
			return nil, fmt.Errorf("Git contains a non-regular SavedVariables entry: %q", file.Name)
		}
		if maxFileSize > 0 && file.Size > maxFileSize {
			continue
		}
		reader, err := file.Reader()
		if err != nil {
			return nil, err
		}
		hash := sha256.New()
		_, readErr := io.Copy(hash, reader)
		closeErr := reader.Close()
		if err := errors.Join(readErr, closeErr); err != nil {
			return nil, err
		}
		var sum [32]byte
		copy(sum[:], hash.Sum(nil))
		versions[filepath.ToSlash(file.Name)] = savedVariableVersion{hash: sum}
	}
	return versions, nil
}

func lastCommitTimeForPath(repo *git.Repository, from plumbing.Hash, path string) (time.Time, error) {
	iterator, err := repo.Log(&git.LogOptions{From: from, FileName: &path})
	if err != nil {
		return time.Time{}, fmt.Errorf("find last Git change for %q: %w", path, err)
	}
	defer iterator.Close()
	commit, err := iterator.Next()
	if err != nil {
		return time.Time{}, fmt.Errorf("find last Git change for %q: %w", path, err)
	}
	return commit.Committer.When, nil
}

func requireGameStopped(probe func() (bool, error)) error {
	running, err := probe()
	if err != nil {
		return fmt.Errorf("could not check whether wow.exe is running: %w", err)
	}
	if running {
		return errors.New("close wow.exe before pulling changes")
	}
	return nil
}

func savedVariablesRelativePath(root, path string) (string, error) {
	relative, err := filepath.Rel(root, path)
	if err != nil {
		return "", err
	}
	return filepath.ToSlash(relative), nil
}

func isRepositoryMetadataPath(path string) bool {
	if strings.Contains(filepath.ToSlash(path), "/") {
		return false
	}
	switch strings.ToLower(path) {
	case ".git", ".gitignore", "license", "license.txt", "readme.md":
		return true
	default:
		return false
	}
}

func savedVariablesDirectoriesHaveSameContents(first, second string, maxFileSize int64) (bool, error) {
	firstFiles, err := savedVariablesFileHashes(first, maxFileSize)
	if err != nil {
		return false, err
	}
	secondFiles, err := savedVariablesFileHashes(second, maxFileSize)
	if err != nil {
		return false, err
	}
	if len(firstFiles) != len(secondFiles) {
		return false, nil
	}
	for path, hash := range firstFiles {
		if secondFiles[path] != hash {
			return false, nil
		}
	}
	return true, nil
}

func savedVariablesFileHashes(root string, maxFileSize int64) (map[string][32]byte, error) {
	files := make(map[string][32]byte)
	err := filepath.Walk(root, func(filePath string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if info.Mode()&os.ModeSymlink != 0 || isReparsePoint(info) {
			return fmt.Errorf("symbolic link is not allowed: %q", filePath)
		}
		relative, err := savedVariablesRelativePath(root, filePath)
		if err != nil {
			return err
		}
		if relative == ".git" && info.IsDir() {
			return filepath.SkipDir
		}
		if relative == "." || !info.Mode().IsRegular() {
			return nil
		}
		if maxFileSize > 0 && info.Size() > maxFileSize {
			return nil
		}
		file, err := os.Open(filePath)
		if err != nil {
			return err
		}
		hash := sha256.New()
		_, readErr := io.Copy(hash, file)
		closeErr := file.Close()
		if err := errors.Join(readErr, closeErr); err != nil {
			return err
		}
		var sum [32]byte
		copy(sum[:], hash.Sum(nil))
		files[relative] = sum
		return nil
	})
	return files, err
}

func securePathUnder(root, relativePath string) (string, error) {
	clean := filepath.Clean(relativePath)
	if filepath.IsAbs(clean) || clean != relativePath || clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("path is outside the permitted directory")
	}
	if err := rejectUnsafePathComponents(root); err != nil {
		return "", err
	}
	rootInfo, err := os.Lstat(root)
	if err != nil {
		return "", err
	}
	if rootInfo.Mode()&os.ModeSymlink != 0 || isReparsePoint(rootInfo) || !rootInfo.IsDir() {
		return "", fmt.Errorf("permitted root is not a regular directory")
	}

	current := root
	for _, part := range strings.Split(clean, string(filepath.Separator)) {
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return "", err
		}
		if info.Mode()&os.ModeSymlink != 0 || isReparsePoint(info) {
			return "", fmt.Errorf("symbolic links are not allowed in synchronized paths")
		}
	}
	relative, err := filepath.Rel(root, current)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) || filepath.IsAbs(relative) {
		return "", fmt.Errorf("path is outside the permitted directory")
	}
	return current, nil
}

func copySavedVariablesFiles(sourceRoot, destinationRoot string, maxFileSize int64) error {
	if err := rejectUnsafePathComponents(sourceRoot); err != nil {
		return err
	}
	if err := rejectUnsafePathComponents(destinationRoot); err != nil {
		return err
	}
	sourceInfo, err := os.Lstat(sourceRoot)
	if err != nil {
		return err
	}
	if sourceInfo.Mode()&os.ModeSymlink != 0 || isReparsePoint(sourceInfo) || !sourceInfo.IsDir() {
		return fmt.Errorf("source is not a regular directory")
	}
	if err := os.MkdirAll(destinationRoot, 0o700); err != nil {
		return err
	}
	destinationInfo, err := os.Lstat(destinationRoot)
	if err != nil {
		return err
	}
	if destinationInfo.Mode()&os.ModeSymlink != 0 || isReparsePoint(destinationInfo) || !destinationInfo.IsDir() {
		return fmt.Errorf("destination is not a regular directory")
	}

	return filepath.Walk(sourceRoot, func(sourcePath string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if info.Mode()&os.ModeSymlink != 0 || isReparsePoint(info) {
			return fmt.Errorf("symbolic link is not allowed: %q", sourcePath)
		}
		relative, err := savedVariablesRelativePath(sourceRoot, sourcePath)
		if err != nil {
			return err
		}
		if relative == ".git" && info.IsDir() {
			return filepath.SkipDir
		}
		if relative == "." {
			return nil
		}
		if isRepositoryMetadataPath(relative) {
			return nil
		}
		destinationPath, err := securePathUnder(destinationRoot, filepath.FromSlash(relative))
		if err != nil {
			return err
		}
		if info.IsDir() {
			return os.MkdirAll(destinationPath, 0o700)
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("selected folder contains a non-regular file: %q", sourcePath)
		}
		if maxFileSize > 0 && info.Size() > maxFileSize {
			return nil
		}
		if err := os.MkdirAll(filepath.Dir(destinationPath), 0o700); err != nil {
			return err
		}
		return CopyFile(sourcePath, destinationPath)
	})
}

func listSavedVariablesFiles(root string) ([]string, error) {
	var paths []string
	err := filepath.Walk(root, func(filePath string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if info.Mode()&os.ModeSymlink != 0 || isReparsePoint(info) {
			return fmt.Errorf("symbolic link is not allowed: %q", filePath)
		}
		relative, err := savedVariablesRelativePath(root, filePath)
		if err != nil {
			return err
		}
		if relative == ".git" && info.IsDir() {
			return filepath.SkipDir
		}
		if relative == "." || !info.Mode().IsRegular() {
			return nil
		}
		if isRepositoryMetadataPath(relative) {
			return nil
		}
		if info.Size() > int64(MaxPushFileSizeMB)*1024*1024 {
			return nil
		}
		paths = append(paths, relative)
		return nil
	})
	return paths, err
}

func savedVariablesFileSizes(root string, maxFileSize int64) (int64, []OversizedFile, error) {
	var total int64
	var oversized []OversizedFile
	err := filepath.Walk(root, func(filePath string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if info.Mode()&os.ModeSymlink != 0 || isReparsePoint(info) {
			return fmt.Errorf("symbolic link is not allowed: %q", filePath)
		}
		relative, err := savedVariablesRelativePath(root, filePath)
		if err != nil {
			return err
		}
		if relative == ".git" && info.IsDir() {
			return filepath.SkipDir
		}
		if relative == "." || !info.Mode().IsRegular() {
			return nil
		}
		if isRepositoryMetadataPath(relative) {
			return nil
		}
		if maxFileSize > 0 && info.Size() > maxFileSize {
			oversized = append(oversized, OversizedFile{Path: filePath, Size: info.Size()})
			return nil
		}
		total += info.Size()
		return nil
	})
	return total, oversized, err
}

func copyTrackedSavedVariables(sourceRoot, destinationRoot string, maxFileSize int64) error {
	repo, err := git.PlainOpen(sourceRoot)
	if err != nil {
		return err
	}
	head, err := repo.Head()
	if err != nil {
		return err
	}
	commit, err := repo.CommitObject(head.Hash())
	if err != nil {
		return err
	}
	tree, err := commit.Tree()
	if err != nil {
		return err
	}
	files := tree.Files()
	defer files.Close()

	for {
		file, err := files.Next()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return err
		}
		if file.Name == "" {
			continue
		}
		if isRepositoryMetadataPath(file.Name) {
			continue
		}
		if !file.Mode.IsRegular() {
			return fmt.Errorf("repository contains a non-regular file: %q", file.Name)
		}
		sourcePath, err := securePathUnder(sourceRoot, filepath.FromSlash(file.Name))
		if err != nil {
			return err
		}
		destinationRelative := filepath.FromSlash(file.Name)
		destinationPath, err := securePathUnder(destinationRoot, destinationRelative)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(destinationPath), 0o700); err != nil {
			return err
		}
		sourceInfo, err := os.Lstat(sourcePath)
		if err != nil {
			return err
		}
		if sourceInfo.Mode()&os.ModeSymlink != 0 || isReparsePoint(sourceInfo) || !sourceInfo.Mode().IsRegular() {
			return fmt.Errorf("tracked file is not a regular file: %q", file.Name)
		}
		if maxFileSize > 0 && sourceInfo.Size() > maxFileSize {
			continue
		}
		destinationInfo, err := os.Lstat(destinationPath)
		if err == nil && maxFileSize > 0 && destinationInfo.Size() > maxFileSize {
			continue
		}
		if err != nil && !os.IsNotExist(err) {
			return err
		}
		if err := CopyFile(sourcePath, destinationPath); err != nil {
			return fmt.Errorf("copy tracked file %q: %w", file.Name, err)
		}
	}
	return nil
}

func trackedRepositoryFiles(repoPath string, maxFileSize int64) ([]string, error) {
	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		return nil, err
	}
	head, err := repo.Head()
	if err != nil {
		return nil, err
	}
	commit, err := repo.CommitObject(head.Hash())
	if err != nil {
		return nil, err
	}
	tree, err := commit.Tree()
	if err != nil {
		return nil, err
	}
	files := tree.Files()
	defer files.Close()

	var paths []string
	for {
		file, err := files.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}
		if !file.Mode.IsRegular() {
			return nil, fmt.Errorf("repository contains a non-regular file: %q", file.Name)
		}
		if isRepositoryMetadataPath(file.Name) {
			continue
		}
		if maxFileSize > 0 && file.Size > maxFileSize {
			continue
		}
		paths = append(paths, filepath.ToSlash(file.Name))
	}
	return paths, nil
}

func removeTrackedFilesMissingFromSource(sourceRoot, repoPath string, maxFileSize int64) error {
	paths, err := trackedRepositoryFiles(repoPath, maxFileSize)
	if err != nil {
		return err
	}
	for _, path := range paths {
		if isRepositoryMetadataPath(path) {
			continue
		}
		sourcePath, err := securePathUnder(sourceRoot, filepath.FromSlash(path))
		if err != nil {
			return err
		}
		if _, err := os.Lstat(sourcePath); err == nil {
			continue
		} else if !os.IsNotExist(err) {
			return err
		}
		repositoryFile, err := securePathUnder(repoPath, filepath.FromSlash(path))
		if err != nil {
			return err
		}
		if err := os.Remove(repositoryFile); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove locally deleted tracked file %q: %w", path, err)
		}
	}
	return nil
}

func removeTrackedFilesDeletedFromRepository(destinationRoot, repoPath string, previousPaths []string, maxFileSize int64) error {
	currentPaths, err := trackedRepositoryFiles(repoPath, maxFileSize)
	if err != nil {
		return err
	}
	current := make(map[string]struct{}, len(currentPaths))
	for _, path := range currentPaths {
		current[path] = struct{}{}
	}
	for _, path := range previousPaths {
		if _, exists := current[path]; exists {
			continue
		}
		destinationPath, err := securePathUnder(destinationRoot, filepath.FromSlash(path))
		if err != nil {
			return err
		}
		if err := os.Remove(destinationPath); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove file deleted from repository %q: %w", path, err)
		}
	}
	return nil
}

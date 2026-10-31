package main

import (
	"archive/zip"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const maxBackupArchives = 5
const maxRestoreSize = 1 << 30

func CreateBackup(accountPath string) (string, error) {
	info, err := os.Lstat(accountPath)
	if err != nil {
		return "", fmt.Errorf("account folder is unavailable: %w", err)
	}
	if err := rejectUnsafePathComponents(accountPath); err != nil {
		return "", err
	}
	if info.Mode()&os.ModeSymlink != 0 || isReparsePoint(info) || !info.IsDir() {
		return "", errors.New("account path must be a regular directory")
	}
	executablePath, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("determine application directory: %w", err)
	}
	backupDir := backupDirectoryForExecutable(executablePath)
	if err := rejectUnsafePathComponents(backupDir); err != nil {
		return "", err
	}
	if err := os.MkdirAll(backupDir, 0o700); err != nil {
		return "", fmt.Errorf("create backup directory %q: %w", backupDir, err)
	}
	backupInfo, err := os.Lstat(backupDir)
	if err != nil {
		return "", err
	}
	if backupInfo.Mode()&os.ModeSymlink != 0 || isReparsePoint(backupInfo) || !backupInfo.IsDir() {
		return "", fmt.Errorf("backup path is not a regular directory: %q", backupDir)
	}
	archivePath := filepath.Join(backupDir, "Account_Backup_"+time.Now().Format("2006-01-02_15-04-05.000000000")+".zip")
	if err := createBackupArchive(accountPath, archivePath); err != nil {
		return "", err
	}
	if err := pruneBackupArchives(backupDir, maxBackupArchives); err != nil {
		return "", fmt.Errorf("remove old backups: %w", err)
	}
	return archivePath, nil
}

func createBackupArchive(accountPath, archivePath string) (returnErr error) {
	temp, err := os.CreateTemp(filepath.Dir(archivePath), "."+filepath.Base(archivePath)+".tmp-*")
	if err != nil {
		return err
	}
	tempPath := temp.Name()
	published := false
	defer func() {
		if !published {
			if removeErr := os.Remove(tempPath); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
				returnErr = errors.Join(returnErr, fmt.Errorf("remove incomplete backup: %w", removeErr))
			}
		}
	}()

	writer := zip.NewWriter(temp)
	walkErr := filepath.Walk(accountPath, func(sourcePath string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if info.Mode()&os.ModeSymlink != 0 || isReparsePoint(info) {
			return fmt.Errorf("symbolic link is not allowed in backup source: %q", sourcePath)
		}
		relative, err := savedVariablesRelativePath(accountPath, sourcePath)
		if err != nil {
			return err
		}
		if relative == "." {
			return nil
		}
		if !info.IsDir() && !info.Mode().IsRegular() {
			return fmt.Errorf("backup source is not a regular file: %q", sourcePath)
		}
		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}
		header.Name = relative
		if info.IsDir() {
			header.Name += "/"
		} else {
			header.Method = zip.Deflate
		}
		entry, err := writer.CreateHeader(header)
		if err != nil || info.IsDir() {
			return err
		}
		source, err := os.Open(sourcePath)
		if err != nil {
			return err
		}
		_, copyErr := io.Copy(entry, source)
		closeErr := source.Close()
		return errors.Join(copyErr, closeErr)
	})
	zipErr := writer.Close()
	if walkErr != nil || zipErr != nil {
		closeErr := temp.Close()
		return errors.Join(walkErr, zipErr, closeErr)
	}
	if err := temp.Sync(); err != nil {
		return errors.Join(err, temp.Close())
	}
	if err := temp.Close(); err != nil {
		return err
	}
	if err := validateBackupArchive(tempPath); err != nil {
		return err
	}
	if err := replaceFileAtomically(tempPath, archivePath); err != nil {
		return fmt.Errorf("publish completed backup: %w", err)
	}
	published = true
	return nil
}

func validateBackupArchive(path string) (returnErr error) {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	info, err := file.Stat()
	if err != nil {
		return errors.Join(err, file.Close())
	}
	archive, err := zip.NewReader(file, info.Size())
	if err != nil {
		return errors.Join(err, file.Close())
	}
	for _, entry := range archive.File {
		reader, err := entry.Open()
		if err != nil {
			return errors.Join(err, file.Close())
		}
		_, readErr := io.Copy(io.Discard, reader)
		closeErr := reader.Close()
		if err := errors.Join(readErr, closeErr); err != nil {
			return errors.Join(err, file.Close())
		}
	}
	return file.Close()
}

func RestoreBackup(archivePath, accountPath string) (string, error) {
	backupPath, err := CreateBackup(accountPath)
	if err != nil {
		return "", fmt.Errorf("create safety backup before restore: %w", err)
	}
	if err := restoreSavedVariablesFromArchive(archivePath, accountPath); err != nil {
		return backupPath, fmt.Errorf("restore archive (safety backup: %q): %w", backupPath, err)
	}
	return backupPath, nil
}

func restoreSavedVariablesFromArchive(archivePath, accountPath string) (returnErr error) {
	if err := rejectUnsafePathComponents(accountPath); err != nil {
		return err
	}
	if err := rejectUnsafePathComponents(archivePath); err != nil {
		return err
	}
	archive, err := zip.OpenReader(archivePath)
	if err != nil {
		return err
	}
	defer func() {
		if err := archive.Close(); err != nil {
			returnErr = errors.Join(returnErr, err)
		}
	}()

	var totalSize uint64
	for _, entry := range archive.File {
		if entry.FileInfo().Mode()&os.ModeSymlink != 0 || isReparsePoint(entry.FileInfo()) {
			return fmt.Errorf("archive contains a symbolic link: %q", entry.Name)
		}
		relative := filepath.FromSlash(entry.Name)
		cleanRelative := strings.TrimSuffix(relative, string(filepath.Separator))
		if filepath.IsAbs(relative) || filepath.Clean(relative) != cleanRelative || cleanRelative == "." || cleanRelative == ".." ||
			strings.HasPrefix(cleanRelative, ".."+string(filepath.Separator)) {
			return fmt.Errorf("archive path is outside the selected folder: %q", entry.Name)
		}
		if entry.FileInfo().IsDir() {
			continue
		}
		if !entry.Mode().IsRegular() {
			return fmt.Errorf("archive contains a non-regular file: %q", entry.Name)
		}
		if entry.UncompressedSize64 > maxRestoreSize-totalSize {
			return fmt.Errorf("archive exceeds the maximum restore size")
		}
		totalSize += entry.UncompressedSize64
		destinationRelative := relative
		destination, err := securePathUnder(accountPath, destinationRelative)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(destination), 0o700); err != nil {
			return err
		}
		source, err := entry.Open()
		if err != nil {
			return err
		}
		copyErr := copyFileAtomically(destination, entry.Mode().Perm(), func(out io.Writer) error {
			_, err := io.Copy(out, source)
			return err
		})
		closeErr := source.Close()
		if err := errors.Join(copyErr, closeErr); err != nil {
			return fmt.Errorf("restore file %q: %w", entry.Name, err)
		}
	}
	return nil
}

func backupDirectoryForExecutable(executablePath string) string {
	return filepath.Join(filepath.Dir(executablePath), "backups")
}

func getBackupDirectory() (string, error) {
	executablePath, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("determine application directory: %w", err)
	}
	directory := backupDirectoryForExecutable(executablePath)
	if err := rejectUnsafePathComponents(directory); err != nil {
		return "", err
	}
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return "", fmt.Errorf("create backup directory: %w", err)
	}
	info, err := os.Lstat(directory)
	if err != nil {
		return "", err
	}
	if info.Mode()&os.ModeSymlink != 0 || isReparsePoint(info) || !info.IsDir() {
		return "", fmt.Errorf("backup path is not a regular directory: %q", directory)
	}
	return directory, nil
}

func pruneBackupArchives(backupDir string, keep int) error {
	if keep < 0 {
		return fmt.Errorf("cannot keep a negative number of archives: %d", keep)
	}
	if err := rejectUnsafePathComponents(backupDir); err != nil {
		return err
	}
	entries, err := os.ReadDir(backupDir)
	if err != nil {
		return err
	}
	type archiveFile struct {
		path    string
		modTime time.Time
		name    string
	}
	archives := make([]archiveFile, 0, len(entries))
	for _, entry := range entries {
		isBackup := strings.HasPrefix(entry.Name(), "Account_Backup_") || strings.HasPrefix(entry.Name(), "SavedVariables_Backup_")
		if entry.IsDir() || !isBackup || filepath.Ext(entry.Name()) != ".zip" {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			return fmt.Errorf("inspect archive %q: %w", entry.Name(), err)
		}
		if info.Mode()&os.ModeSymlink != 0 || isReparsePoint(info) || !info.Mode().IsRegular() {
			return fmt.Errorf("backup archive is not a regular file: %q", entry.Name())
		}
		archives = append(archives, archiveFile{filepath.Join(backupDir, entry.Name()), info.ModTime(), entry.Name()})
	}
	sort.Slice(archives, func(i, j int) bool {
		if archives[i].modTime.Equal(archives[j].modTime) {
			return archives[i].name < archives[j].name
		}
		return archives[i].modTime.Before(archives[j].modTime)
	})
	removeCount := len(archives) - keep
	if removeCount < 0 {
		removeCount = 0
	}
	for _, archive := range archives[:removeCount] {
		if err := os.Remove(archive.path); err != nil {
			return fmt.Errorf("remove archive %q: %w", archive.path, err)
		}
	}
	return nil
}

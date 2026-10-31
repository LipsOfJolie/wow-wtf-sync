package main

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

var ignoredNames = map[string]bool{
	".git":       true,
	".gitignore": true,
	"LICENSE":    true,
	"README.md":  true,
}

// Копирует только имеющиеся файлы и никогда не удаляет данные из назначения.
func CopyDir(src string, dst string) error {
	if err := rejectUnsafePathComponents(src); err != nil {
		return err
	}
	if err := rejectUnsafePathComponents(dst); err != nil {
		return err
	}
	srcInfo, err := os.Lstat(src)
	if err != nil {
		return err
	}
	if srcInfo.Mode()&os.ModeSymlink != 0 || isReparsePoint(srcInfo) || !srcInfo.IsDir() {
		return errors.New("source must be a regular directory")
	}
	if err := os.MkdirAll(dst, 0o700); err != nil {
		return err
	}
	dstInfo, err := os.Lstat(dst)
	if err != nil {
		return err
	}
	if dstInfo.Mode()&os.ModeSymlink != 0 || isReparsePoint(dstInfo) || !dstInfo.IsDir() {
		return errors.New("destination must be a regular directory")
	}
	return filepath.Walk(src, func(sourcePath string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if info.Mode()&os.ModeSymlink != 0 || isReparsePoint(info) {
			return errors.New("symbolic links are not allowed in copied directories")
		}
		relative, err := filepath.Rel(src, sourcePath)
		if err != nil {
			return err
		}
		if relative == "." {
			return nil
		}
		if ignoredNames[filepath.Base(relative)] {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		destinationPath, err := securePathUnder(dst, relative)
		if err != nil {
			return err
		}
		if info.IsDir() {
			return os.MkdirAll(destinationPath, 0o700)
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		if err := os.MkdirAll(filepath.Dir(destinationPath), 0o700); err != nil {
			return err
		}
		return CopyFile(sourcePath, destinationPath)
	})
}

func directoriesHaveSameContents(first, second string) (bool, error) {
	firstEntries, err := os.ReadDir(first)
	if err != nil {
		return false, err
	}
	secondEntries, err := os.ReadDir(second)
	if err != nil {
		return false, err
	}

	filteredFirst := make([]os.DirEntry, 0, len(firstEntries))
	for _, entry := range firstEntries {
		if !ignoredNames[entry.Name()] {
			filteredFirst = append(filteredFirst, entry)
		}
	}
	filteredSecond := make([]os.DirEntry, 0, len(secondEntries))
	for _, entry := range secondEntries {
		if !ignoredNames[entry.Name()] {
			filteredSecond = append(filteredSecond, entry)
		}
	}
	if len(filteredFirst) != len(filteredSecond) {
		return false, nil
	}

	for i, firstEntry := range filteredFirst {
		secondEntry := filteredSecond[i]
		if firstEntry.Name() != secondEntry.Name() || firstEntry.IsDir() != secondEntry.IsDir() {
			return false, nil
		}

		firstPath := filepath.Join(first, firstEntry.Name())
		secondPath := filepath.Join(second, secondEntry.Name())
		if firstEntry.IsDir() {
			same, err := directoriesHaveSameContents(firstPath, secondPath)
			if err != nil || !same {
				return same, err
			}
			continue
		}

		firstFile, err := os.Open(firstPath)
		if err != nil {
			return false, err
		}
		secondFile, err := os.Open(secondPath)
		if err != nil {
			return false, errors.Join(err, firstFile.Close())
		}
		firstHash, secondHash := sha256.New(), sha256.New()
		_, firstErr := io.Copy(firstHash, firstFile)
		_, secondErr := io.Copy(secondHash, secondFile)
		firstCloseErr := firstFile.Close()
		secondCloseErr := secondFile.Close()
		if firstErr != nil {
			return false, firstErr
		}
		if secondErr != nil {
			return false, secondErr
		}
		if firstCloseErr != nil {
			return false, firstCloseErr
		}
		if secondCloseErr != nil {
			return false, secondCloseErr
		}
		if !bytes.Equal(firstHash.Sum(nil), secondHash.Sum(nil)) {
			return false, nil
		}
	}
	return true, nil
}

func CopyFile(src, dst string) error {
	if err := rejectUnsafePathComponents(src); err != nil {
		return err
	}
	sourceInfo, err := os.Lstat(src)
	if err != nil {
		return err
	}
	if sourceInfo.Mode()&os.ModeSymlink != 0 || isReparsePoint(sourceInfo) || !sourceInfo.Mode().IsRegular() {
		return errors.New("source is not a regular file")
	}
	return copyFileAtomically(dst, sourceInfo.Mode().Perm(), func(out io.Writer) error {
		in, err := os.Open(src)
		if err != nil {
			return err
		}
		_, copyErr := io.Copy(out, in)
		closeErr := in.Close()
		return errors.Join(copyErr, closeErr)
	})
}

func copyFileAtomically(destination string, mode os.FileMode, write func(io.Writer) error) (returnErr error) {
	if err := rejectUnsafePathComponents(filepath.Dir(destination)); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(destination), 0o700); err != nil {
		return err
	}
	if info, err := os.Lstat(destination); err == nil {
		if info.Mode()&os.ModeSymlink != 0 || isReparsePoint(info) || !info.Mode().IsRegular() {
			return errors.New("destination is not a regular file")
		}
	} else if !os.IsNotExist(err) {
		return err
	}

	temp, err := os.CreateTemp(filepath.Dir(destination), "."+filepath.Base(destination)+".tmp-*")
	if err != nil {
		return err
	}
	tempPath := temp.Name()
	replaced := false
	defer func() {
		if !replaced {
			if err := os.Remove(tempPath); err != nil && !os.IsNotExist(err) {
				returnErr = errors.Join(returnErr, fmt.Errorf("remove temporary file: %w", err))
			}
		}
	}()

	if err := temp.Chmod(mode); err != nil {
		returnErr = errors.Join(err, temp.Close())
		return returnErr
	}
	if err := write(temp); err != nil {
		returnErr = errors.Join(err, temp.Close())
		return returnErr
	}
	if err := temp.Sync(); err != nil {
		returnErr = errors.Join(err, temp.Close())
		return returnErr
	}
	if err := temp.Close(); err != nil {
		return err
	}
	if err := replaceFileAtomically(tempPath, destination); err != nil {
		return err
	}
	replaced = true
	return nil
}

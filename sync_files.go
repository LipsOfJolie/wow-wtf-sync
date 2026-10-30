package main

import (
	"io"
	"os"
	"path/filepath"
	"strings"
)

// Список файлов и папок, которые никому нельзя копировать в папку игры
var ignoredNames = map[string]bool{
	".gitignore": true,
	"LICENSE":    true,
	"README.md":  true,
}

// Полная синхронизация папок (с удалением файлов, которых больше нет в источнике)
func CopyDir(src string, dst string) error {
	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}

	if err = os.MkdirAll(dst, srcInfo.Mode()); err != nil {
		return err
	}

	// 1. Читаем содержимое исходной папки
	srcEntries, err := os.ReadDir(src)
	if err != nil {
		return err
	}

	srcMap := make(map[string]bool)
	for _, entry := range srcEntries {
		// Если мы находимся внутри локального репозитория, ни в коем случае не трогаем .git
		if entry.Name() == ".git" {
			continue
		}
		if ignoredNames[entry.Name()] {
			continue
		}
		srcMap[entry.Name()] = true
	}

	// 2. Читаем содержимое целевой папки, чтобы удалить лишнее (то, чего нет в src)
	dstEntries, err := os.ReadDir(dst)
	if err == nil {
		for _, entry := range dstEntries {
			// Никогда не удаляем .git папки!
			if entry.Name() == ".git" {
				continue
			}

			if ignoredNames[entry.Name()] {
				_ = os.RemoveAll(filepath.Join(dst, entry.Name()))
				continue
			}

			// Если файл/папка есть в назначении, но исчезла из источника — удаляем её
			if !srcMap[entry.Name()] {
				fullDstPath := filepath.Join(dst, entry.Name())
				_ = os.RemoveAll(fullDstPath)
			}
		}
	}

	// 3. Копируем актуальные файлы и папки
	for _, entry := range srcEntries {
		if entry.Name() == ".git" {
			continue
		}
		if ignoredNames[entry.Name()] {
			continue
		}

		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		if entry.IsDir() {
			if err = CopyDir(srcPath, dstPath); err != nil {
				return err
			}
		} else {
			if err = CopyFile(srcPath, dstPath); err != nil {
				return err
			}
		}
	}

	// Дополнительная защита для путей с Account
	if strings.Contains(dst, "Account") {
		entries, _ := os.ReadDir(dst)
		for _, e := range entries {
			if e.Name() == ".git" {
				continue
			}
			if !e.IsDir() && ignoredNames[e.Name()] {
				_ = os.Remove(filepath.Join(dst, e.Name()))
			}
		}
	}

	return nil
}

func CopyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err = io.Copy(out, in); err != nil {
		return err
	}

	info, err := os.Stat(src)
	if err != nil {
		return err
	}
	return os.Chmod(dst, info.Mode())
}

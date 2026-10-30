package main

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

// Создает zip-архив папки SavedVariables в директории ./backups/
func CreateBackup(gamePath string) (string, error) {
	// Путь к папке SavedVariables в игре (учитываем структуру WTF\Account\...)
	// Для универсальности ищем папки SavedVariables внутри Account
	accountPath := gamePath
	if _, err := os.Stat(accountPath); os.IsNotExist(err) {
		return "", fmt.Errorf("папка Account не найдена по пути: %s", accountPath)
	}

	// Создаем папку backups рядом с лаунчером, если её нет
	backupDir := "./backups"
	if err := os.MkdirAll(backupDir, os.ModePerm); err != nil {
		return "", fmt.Errorf("не удалось создать папку для бэкапов: %w", err)
	}

	// Имя архива с датой и временем
	currentTime := time.Now().Format("2006-01-02_15-04-05")
	zipFileName := filepath.Join(backupDir, fmt.Sprintf("SavedVariables_Backup_%s.zip", currentTime))

	// Создаем zip-файл
	zipFile, err := os.Create(zipFileName)
	if err != nil {
		return "", fmt.Errorf("не удалось создать файл архива: %w", err)
	}
	defer zipFile.Close()

	archive := zip.NewWriter(zipFile)
	defer archive.Close()

	// Рекурсивно добавляем файлы из WTF\Account в архив
	err = filepath.Walk(accountPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Пропускаем все файлы, кроме SavedVariables (или архивируем всю Account, но без кэша)
		// Нам нужны именно файлы с настройками аддонов
		relPath, err := filepath.Rel(accountPath, path)
		if err != nil {
			return err
		}

		// Создаем заголовки для архива
		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}
		header.Name = relPath

		if info.IsDir() {
			header.Name += "/"
		} else {
			header.Method = zip.Deflate
		}

		writer, err := archive.CreateHeader(header)
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()

		_, err = io.Copy(writer, file)
		return err
	})

	if err != nil {
		return "", fmt.Errorf("ошибка при упаковке в архив: %w", err)
	}

	return zipFileName, nil
}

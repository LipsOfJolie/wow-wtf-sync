package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

type storedConfig struct {
	GamePath       string `json:"game_path"`
	RepoURL        string `json:"repo_url"`
	GitHubToken    string `json:"github_token,omitempty"`
	EncryptedToken string `json:"github_token_encrypted,omitempty"`
	StartMinimized bool   `json:"start_minimized"`
	AutoSync       bool   `json:"auto_sync"`
	Autostart      bool   `json:"autostart"`
	LastSyncTime   string `json:"last_sync_time,omitempty"`
	LastSyncStatus string `json:"last_sync_status,omitempty"`
}

func getConfigPath() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("determine user configuration directory: %w", err)
	}
	return filepath.Join(configDir, "WoWWTFSync", ConfigFileName), nil
}

func legacyConfigPath() (string, error) {
	executablePath, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("determine application directory: %w", err)
	}
	return filepath.Join(filepath.Dir(executablePath), ConfigFileName), nil
}

func loadConfig() (*Config, error) {
	configPath, err := getConfigPath()
	if err != nil {
		return nil, err
	}
	if err := rejectUnsafePathComponents(configPath); err != nil {
		return nil, fmt.Errorf("unsafe configuration path: %w", err)
	}
	data, err := os.ReadFile(configPath)
	if errors.Is(err, os.ErrNotExist) {
		oldPath, pathErr := legacyConfigPath()
		if pathErr != nil {
			return nil, pathErr
		}
		if err := rejectUnsafePathComponents(oldPath); err != nil {
			return nil, fmt.Errorf("unsafe legacy configuration path: %w", err)
		}
		data, err = os.ReadFile(oldPath)
		if errors.Is(err, os.ErrNotExist) {
			return nil, os.ErrNotExist
		}
		if err != nil {
			return nil, fmt.Errorf("read legacy configuration: %w", err)
		}
		config, decodeErr := decodeConfig(data, unprotectSecret, true)
		if decodeErr != nil {
			return nil, decodeErr
		}
		if err := saveConfig(config); err != nil {
			return config, fmt.Errorf("migrate configuration to user profile: %w", err)
		}
		if err := os.Remove(oldPath); err != nil && !errors.Is(err, os.ErrNotExist) {
			return config, fmt.Errorf("remove legacy configuration after migration: %w", err)
		}
		return config, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read configuration: %w", err)
	}
	return decodeConfig(data, unprotectSecret, false)
}

func decodeConfig(data []byte, unprotect func(string) (string, error), allowLegacyToken bool) (*Config, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var stored storedConfig
	if err := decoder.Decode(&stored); err != nil {
		return nil, fmt.Errorf("decode configuration: %w", err)
	}
	if err := decoder.Decode(new(any)); !errors.Is(err, io.EOF) {
		return nil, errors.New("configuration contains trailing data")
	}

	token := stored.GitHubToken
	if stored.EncryptedToken != "" {
		if token != "" {
			return nil, errors.New("configuration contains both plaintext and encrypted GitHub tokens")
		}
		decrypted, err := unprotect(stored.EncryptedToken)
		if err != nil {
			return nil, fmt.Errorf("decrypt stored GitHub token: %w", err)
		}
		token = decrypted
	} else if token != "" && !allowLegacyToken {
		return nil, errors.New("configuration contains an unencrypted GitHub token")
	}
	return &Config{
		GamePath:       stored.GamePath,
		RepoURL:        stored.RepoURL,
		GitHubToken:    token,
		StartMinimized: stored.StartMinimized,
		AutoSync:       stored.AutoSync,
		Autostart:      stored.Autostart,
		LastSyncTime:   stored.LastSyncTime,
		LastSyncStatus: stored.LastSyncStatus,
	}, nil
}

func saveConfig(config *Config) error {
	configPath, err := getConfigPath()
	if err != nil {
		return err
	}
	data, err := marshalConfig(config, protectSecret)
	if err != nil {
		return err
	}
	if err := writeFileAtomically(configPath, data, 0o600); err != nil {
		return fmt.Errorf("write configuration: %w", err)
	}
	return nil
}

func marshalConfig(config *Config, protect func(string) (string, error)) ([]byte, error) {
	var encryptedToken string
	if config.GitHubToken != "" {
		var err error
		encryptedToken, err = protect(config.GitHubToken)
		if err != nil {
			return nil, fmt.Errorf("encrypt GitHub token: %w", err)
		}
	}

	data, err := json.MarshalIndent(storedConfig{
		GamePath:       config.GamePath,
		RepoURL:        config.RepoURL,
		EncryptedToken: encryptedToken,
		StartMinimized: config.StartMinimized,
		AutoSync:       config.AutoSync,
		Autostart:      config.Autostart,
		LastSyncTime:   config.LastSyncTime,
		LastSyncStatus: config.LastSyncStatus,
	}, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("encode configuration: %w", err)
	}
	data = append(data, '\n')
	return data, nil
}

func writeFileAtomically(path string, data []byte, mode os.FileMode) (returnErr error) {
	if err := rejectUnsafePathComponents(filepath.Dir(path)); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	file, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	tempPath := file.Name()
	keepTemp := false
	defer func() {
		if !keepTemp {
			if err := os.Remove(tempPath); err != nil && !errors.Is(err, os.ErrNotExist) {
				returnErr = errors.Join(returnErr, fmt.Errorf("remove temporary file: %w", err))
			}
		}
	}()

	if err := file.Chmod(mode); err != nil {
		closeErr := file.Close()
		if closeErr != nil {
			return errors.Join(err, closeErr)
		}
		return err
	}
	written, err := file.Write(data)
	if err != nil {
		closeErr := file.Close()
		if closeErr != nil {
			return errors.Join(err, closeErr)
		}
		return err
	}
	if written != len(data) {
		closeErr := file.Close()
		return errors.Join(io.ErrShortWrite, closeErr)
	}
	if err := file.Sync(); err != nil {
		closeErr := file.Close()
		if closeErr != nil {
			return errors.Join(err, closeErr)
		}
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	if err := replaceFileAtomically(tempPath, path); err != nil {
		return err
	}
	keepTemp = true
	return nil
}

package main

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
)

func TestConfigStorageEncryptsTokenField(t *testing.T) {
	const token = "github-pat-that-must-not-be-plain"
	data, err := marshalConfig(&Config{GitHubToken: token}, func(value string) (string, error) {
		return base64.StdEncoding.EncodeToString([]byte(value)), nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), token) {
		t.Fatal("serialized config contains the plaintext token")
	}

	var stored storedConfig
	if err := json.Unmarshal(data, &stored); err != nil {
		t.Fatal(err)
	}
	if stored.GitHubToken != "" || stored.EncryptedToken == "" {
		t.Fatalf("stored token fields are not encrypted: %#v", stored)
	}
	config, err := decodeConfig(data, func(value string) (string, error) {
		decoded, err := base64.StdEncoding.DecodeString(value)
		return string(decoded), err
	}, false)
	if err != nil {
		t.Fatal(err)
	}
	if config.GitHubToken != token {
		t.Fatal("encrypted token did not round-trip")
	}
}

func TestConfigStorageRejectsPlaintextTokenOutsideMigration(t *testing.T) {
	data := []byte(`{"github_token":"plaintext"}`)
	if _, err := decodeConfig(data, func(value string) (string, error) {
		return value, nil
	}, false); err == nil {
		t.Fatal("plaintext token was accepted in current config")
	}
	data = []byte(`{"github_token":"plaintext","github_token_encrypted":"encoded"}`)
	if _, err := decodeConfig(data, func(value string) (string, error) {
		return value, nil
	}, true); err == nil {
		t.Fatal("configuration containing both plaintext and encrypted tokens was accepted")
	}
}

func TestConfigStorageMigrationEncryptsLegacyToken(t *testing.T) {
	data := []byte(`{"github_token":"legacy-token"}`)
	config, err := decodeConfig(data, func(value string) (string, error) {
		return value, nil
	}, true)
	if err != nil {
		t.Fatal(err)
	}
	serialized, err := marshalConfig(config, func(value string) (string, error) {
		return base64.StdEncoding.EncodeToString([]byte(value)), nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(serialized), "legacy-token") {
		t.Fatal("migrated config contains the plaintext legacy token")
	}
}

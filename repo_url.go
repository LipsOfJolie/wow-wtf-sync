package main

import (
	"fmt"
	"net/url"
	"path"
	"strings"
)

func validateGitHubRepoURL(raw string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return "", fmt.Errorf("invalid repository URL")
	}
	validHost := strings.EqualFold(parsed.Host, "github.com") || strings.EqualFold(parsed.Host, "github.com:443")
	if !strings.EqualFold(parsed.Scheme, "https") || !validHost || parsed.User != nil ||
		parsed.RawQuery != "" || parsed.ForceQuery || parsed.Fragment != "" || strings.Contains(raw, "#") ||
		parsed.Opaque != "" || parsed.RawPath != "" {
		return "", fmt.Errorf("repository URL must be an HTTPS GitHub URL")
	}

	repositoryPath := strings.Trim(parsed.Path, "/")
	parts := strings.Split(repositoryPath, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", fmt.Errorf("repository URL must contain an owner and repository")
	}
	name := strings.TrimSuffix(parts[1], ".git")
	if name == "" || parts[0] == "." || parts[0] == ".." || name == "." || name == ".." ||
		path.Clean(repositoryPath) != repositoryPath || strings.ContainsAny(parts[0]+name, `\:`) {
		return "", fmt.Errorf("repository URL contains an invalid path")
	}
	for _, character := range parts[0] + name {
		if !((character >= 'a' && character <= 'z') || (character >= 'A' && character <= 'Z') ||
			(character >= '0' && character <= '9') || character == '-' || character == '_' || character == '.') {
			return "", fmt.Errorf("repository URL contains an invalid path")
		}
	}
	return "https://github.com/" + parts[0] + "/" + name + ".git", nil
}

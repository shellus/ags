package environment

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/shellus/ags/internal/command"
)

type RepositoryManager struct {
	CacheDir string
	Runner   command.Runner
}

func (m RepositoryManager) runner() command.Runner {
	if m.Runner == nil {
		return command.OSRunner{}
	}
	return m.Runner
}

func (m RepositoryManager) Sync(source, branch string) (string, string, error) {
	if strings.TrimSpace(source) == "" {
		return "", "", fmt.Errorf("environment source is not configured")
	}
	if info, err := os.Stat(source); err == nil && info.IsDir() {
		root, err := filepath.Abs(source)
		if err != nil {
			return "", "", err
		}
		commit := "local"
		if output, err := m.runner().Run(root, nil, "git", "rev-parse", "HEAD"); err == nil {
			commit = strings.TrimSpace(string(output))
		}
		return root, commit, nil
	}
	if strings.TrimSpace(branch) == "" {
		branch = "main"
	}
	digest := fmt.Sprintf("%x", sha256.Sum256([]byte(source)))[:16]
	checkout := filepath.Join(m.CacheDir, "environment", digest)
	if err := os.MkdirAll(filepath.Dir(checkout), 0o700); err != nil {
		return "", "", fmt.Errorf("create environment cache: %w", err)
	}
	if _, err := os.Stat(filepath.Join(checkout, ".git")); os.IsNotExist(err) {
		if err := os.RemoveAll(checkout); err != nil {
			return "", "", err
		}
		if _, err := m.runner().Run("", nil, "git", "clone", "--no-checkout", source, checkout); err != nil {
			return "", "", fmt.Errorf("clone environment repository: %w", err)
		}
	} else if err != nil {
		return "", "", fmt.Errorf("inspect environment checkout: %w", err)
	}
	remote, err := m.runner().Run(checkout, nil, "git", "remote", "get-url", "origin")
	if err != nil {
		return "", "", err
	}
	if strings.TrimSpace(string(remote)) != source {
		return "", "", fmt.Errorf("cached environment remote is %q, expected %q", strings.TrimSpace(string(remote)), source)
	}
	if _, err := m.runner().Run(checkout, nil, "git", "fetch", "--prune", "origin", branch); err != nil {
		return "", "", fmt.Errorf("fetch environment branch %s: %w", branch, err)
	}
	if _, err := m.runner().Run(checkout, nil, "git", "checkout", "--detach", "--force", "FETCH_HEAD"); err != nil {
		return "", "", fmt.Errorf("checkout environment branch %s: %w", branch, err)
	}
	if _, err := m.runner().Run(checkout, nil, "git", "clean", "-fdx"); err != nil {
		return "", "", fmt.Errorf("clean environment checkout: %w", err)
	}
	output, err := m.runner().Run(checkout, nil, "git", "rev-parse", "HEAD")
	if err != nil {
		return "", "", err
	}
	return checkout, strings.TrimSpace(string(output)), nil
}

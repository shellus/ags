package configfile

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Paths struct {
	Registry       string
	CodexAuth      string
	CodexConfig    string
	ClaudeSettings string
}

func ResolvePaths() (Paths, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return Paths{}, fmt.Errorf("resolve user home directory: %w", err)
	}
	return ResolvePathsWith(home, os.Getenv)
}

func ResolvePathsWith(home string, getenv func(string) string) (Paths, error) {
	if strings.TrimSpace(home) == "" {
		return Paths{}, fmt.Errorf("user home directory is empty")
	}

	codexDir := resolveDirectory(home, getenv("CODEX_HOME"), ".codex")
	claudeDir := resolveDirectory(home, getenv("CLAUDE_CONFIG_DIR"), ".claude")

	return Paths{
		Registry:       filepath.Join(home, ".agent-switch", "providers.yaml"),
		CodexAuth:      filepath.Join(codexDir, "auth.json"),
		CodexConfig:    filepath.Join(codexDir, "config.toml"),
		ClaudeSettings: filepath.Join(claudeDir, "settings.json"),
	}, nil
}

func resolveDirectory(home, override, fallback string) string {
	override = strings.TrimSpace(override)
	if override == "" {
		return filepath.Join(home, fallback)
	}
	if override == "~" {
		return home
	}
	if strings.HasPrefix(override, "~/") || strings.HasPrefix(override, `~\`) {
		return filepath.Join(home, override[2:])
	}
	return override
}

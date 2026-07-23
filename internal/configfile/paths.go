package configfile

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Paths struct {
	Home           string
	ConfigDir      string
	CacheDir       string
	StateDir       string
	AGSConfig      string
	Registry       string
	CodexDir       string
	CodexAuth      string
	CodexConfig    string
	CodexGuidance  string
	CodexSkills    string
	ClaudeDir      string
	ClaudeSettings string
	ClaudeGuidance string
	ClaudeSkills   string
}

func ResolvePaths() (Paths, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return Paths{}, fmt.Errorf("resolve user home directory: %w", err)
	}
	systemConfigDir, err := os.UserConfigDir()
	if err != nil {
		return Paths{}, fmt.Errorf("resolve user config directory: %w", err)
	}
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		return Paths{}, fmt.Errorf("resolve user cache directory: %w", err)
	}
	stateDir := resolveStateDir(home, systemConfigDir, os.Getenv)
	return ResolvePathsWith(home, cacheDir, stateDir, os.Getenv)
}

func ResolvePathsWith(home, cacheRoot, stateRoot string, getenv func(string) string) (Paths, error) {
	if strings.TrimSpace(home) == "" {
		return Paths{}, fmt.Errorf("user home directory is empty")
	}
	for label, value := range map[string]string{"cache": cacheRoot, "state": stateRoot} {
		if strings.TrimSpace(value) == "" {
			return Paths{}, fmt.Errorf("user %s directory is empty", label)
		}
	}

	codexDir := resolveDirectory(home, getenv("CODEX_HOME"), ".codex")
	claudeDir := resolveDirectory(home, getenv("CLAUDE_CONFIG_DIR"), ".claude")
	agsConfigDir := filepath.Join(home, ".ags")
	agsCacheDir := filepath.Join(cacheRoot, "ags")
	agsStateDir := filepath.Join(stateRoot, "ags")

	return Paths{
		Home:           home,
		ConfigDir:      agsConfigDir,
		CacheDir:       agsCacheDir,
		StateDir:       agsStateDir,
		AGSConfig:      filepath.Join(agsConfigDir, "config.yaml"),
		Registry:       filepath.Join(agsConfigDir, "providers.yaml"),
		CodexDir:       codexDir,
		CodexAuth:      filepath.Join(codexDir, "auth.json"),
		CodexConfig:    filepath.Join(codexDir, "config.toml"),
		CodexGuidance:  filepath.Join(codexDir, "AGENTS.md"),
		CodexSkills:    filepath.Join(codexDir, "skills"),
		ClaudeDir:      claudeDir,
		ClaudeSettings: filepath.Join(claudeDir, "settings.json"),
		ClaudeGuidance: filepath.Join(claudeDir, "CLAUDE.md"),
		ClaudeSkills:   filepath.Join(claudeDir, "skills"),
	}, nil
}

func resolveStateDir(home, configDir string, getenv func(string) string) string {
	if value := strings.TrimSpace(getenv("AGS_STATE_DIR")); value != "" {
		return resolveDirectory(home, value, filepath.Join(".local", "state"))
	}
	if value := strings.TrimSpace(getenv("XDG_STATE_HOME")); value != "" {
		return resolveDirectory(home, value, filepath.Join(".local", "state"))
	}
	if strings.Contains(strings.ToLower(configDir), "appdata") {
		return configDir
	}
	return filepath.Join(home, ".local", "state")
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

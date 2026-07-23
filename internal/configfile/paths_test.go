package configfile

import (
	"path/filepath"
	"testing"
)

func TestResolvePathsWithDefaults(t *testing.T) {
	home := filepath.Join(string(filepath.Separator), "users", "tester")
	configRoot := filepath.Join(home, ".config")
	cacheRoot := filepath.Join(home, ".cache")
	stateRoot := filepath.Join(home, ".local", "state")
	paths, err := ResolvePathsWith(home, configRoot, cacheRoot, stateRoot, func(string) string { return "" })
	if err != nil {
		t.Fatal(err)
	}

	assertPath(t, paths.Registry, filepath.Join(configRoot, "ags", "providers.yaml"))
	assertPath(t, paths.AGSConfig, filepath.Join(configRoot, "ags", "config.yaml"))
	assertPath(t, paths.CacheDir, filepath.Join(cacheRoot, "ags"))
	assertPath(t, paths.StateDir, filepath.Join(stateRoot, "ags"))
	assertPath(t, paths.CodexAuth, filepath.Join(home, ".codex", "auth.json"))
	assertPath(t, paths.CodexConfig, filepath.Join(home, ".codex", "config.toml"))
	assertPath(t, paths.CodexGuidance, filepath.Join(home, ".codex", "AGENTS.md"))
	assertPath(t, paths.CodexSkills, filepath.Join(home, ".codex", "skills"))
	assertPath(t, paths.ClaudeSettings, filepath.Join(home, ".claude", "settings.json"))
	assertPath(t, paths.ClaudeGuidance, filepath.Join(home, ".claude", "CLAUDE.md"))
	assertPath(t, paths.ClaudeSkills, filepath.Join(home, ".claude", "skills"))
}

func TestResolvePathsWithOverrides(t *testing.T) {
	home := filepath.Join(string(filepath.Separator), "users", "tester")
	env := map[string]string{
		"CODEX_HOME":        filepath.Join(string(filepath.Separator), "config", "codex"),
		"CLAUDE_CONFIG_DIR": "~/.config/claude",
	}
	paths, err := ResolvePathsWith(
		home,
		filepath.Join(home, ".config"),
		filepath.Join(home, ".cache"),
		filepath.Join(home, ".local", "state"),
		func(key string) string { return env[key] },
	)
	if err != nil {
		t.Fatal(err)
	}

	assertPath(t, paths.CodexAuth, filepath.Join(string(filepath.Separator), "config", "codex", "auth.json"))
	assertPath(t, paths.ClaudeSettings, filepath.Join(home, ".config", "claude", "settings.json"))
}

func assertPath(t *testing.T, actual, expected string) {
	t.Helper()
	if actual != expected {
		t.Fatalf("path = %q, want %q", actual, expected)
	}
}

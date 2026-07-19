package configfile

import (
	"path/filepath"
	"testing"
)

func TestResolvePathsWithDefaults(t *testing.T) {
	home := filepath.Join(string(filepath.Separator), "users", "tester")
	paths, err := ResolvePathsWith(home, func(string) string { return "" })
	if err != nil {
		t.Fatal(err)
	}

	assertPath(t, paths.Registry, filepath.Join(home, ".agent-switch", "providers.yaml"))
	assertPath(t, paths.CodexAuth, filepath.Join(home, ".codex", "auth.json"))
	assertPath(t, paths.CodexConfig, filepath.Join(home, ".codex", "config.toml"))
	assertPath(t, paths.ClaudeSettings, filepath.Join(home, ".claude", "settings.json"))
}

func TestResolvePathsWithOverrides(t *testing.T) {
	home := filepath.Join(string(filepath.Separator), "users", "tester")
	env := map[string]string{
		"CODEX_HOME":        filepath.Join(string(filepath.Separator), "config", "codex"),
		"CLAUDE_CONFIG_DIR": "~/.config/claude",
	}
	paths, err := ResolvePathsWith(home, func(key string) string { return env[key] })
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

package switcher

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shellus/ags/internal/configfile"
	"github.com/shellus/ags/internal/registry"
)

func TestSwitchAllUpdatesOnlyProviderFields(t *testing.T) {
	paths := testPaths(t)
	writeFile(t, paths.CodexAuth, `{
  "OPENAI_API_KEY": "old-codex-key",
  "tokens": {"access_token": "keep-auth-field"}
}
`)
	writeFile(t, paths.CodexConfig, `model_provider = "custom"
model = "keep-model"

[model_providers.other]
base_url = "https://keep.example/v1"

[model_providers.custom]
name = "Custom"
base_url = "https://old.example/v1"
wire_api = "responses"
`)
	writeFile(t, paths.ClaudeSettings, `{
  "model": "keep-model",
  "env": {
    "ANTHROPIC_AUTH_TOKEN": "old-claude-key",
    "ANTHROPIC_BASE_URL": "https://old.example",
    "IS_SANDBOX": "1"
  },
  "permissions": {
    "defaultMode": "acceptEdits"
  }
}
`)

	providerRegistry := &registry.Registry{
		Version: registry.CurrentVersion,
		Providers: map[string]registry.Provider{
			"relay": {
				Codex:  &registry.CodexProvider{APIKey: "new-codex-key", BaseURL: "https://codex.example/v1"},
				Claude: &registry.ClaudeProvider{AuthToken: "new-claude-key", BaseURL: "https://claude.example"},
			},
		},
	}
	service := Service{Paths: paths, Registry: providerRegistry}

	if err := service.Switch(AgentAll, "relay"); err != nil {
		t.Fatal(err)
	}

	auth := readJSON(t, paths.CodexAuth)
	if auth["OPENAI_API_KEY"] != "new-codex-key" {
		t.Fatalf("Codex key = %#v", auth["OPENAI_API_KEY"])
	}
	if auth["tokens"].(map[string]any)["access_token"] != "keep-auth-field" {
		t.Fatal("Codex unrelated auth field changed")
	}

	config := readFile(t, paths.CodexConfig)
	if !strings.Contains(config, `base_url = "https://codex.example/v1"`) {
		t.Fatalf("Codex custom base URL was not updated:\n%s", config)
	}
	if !strings.Contains(config, `base_url = "https://keep.example/v1"`) || !strings.Contains(config, `model = "keep-model"`) {
		t.Fatalf("Codex unrelated config changed:\n%s", config)
	}

	claude := readJSON(t, paths.ClaudeSettings)
	env := claude["env"].(map[string]any)
	if env["ANTHROPIC_AUTH_TOKEN"] != "new-claude-key" || env["ANTHROPIC_BASE_URL"] != "https://claude.example" {
		t.Fatalf("Claude provider fields = %#v", env)
	}
	if env["IS_SANDBOX"] != "1" || claude["model"] != "keep-model" {
		t.Fatal("Claude unrelated settings changed")
	}

	state, err := service.Current()
	if err != nil {
		t.Fatal(err)
	}
	if len(state.Codex) != 1 || state.Codex[0] != "relay" || len(state.Claude) != 1 || state.Claude[0] != "relay" {
		t.Fatalf("Current() = %#v", state)
	}
}

func TestSwitchAllValidatesBothAgentsBeforeWriting(t *testing.T) {
	paths := testPaths(t)
	writeFile(t, paths.CodexAuth, `{"OPENAI_API_KEY":"old"}
`)
	writeFile(t, paths.CodexConfig, `[model_providers.custom]
base_url = "old"
`)
	originalAuth := readFile(t, paths.CodexAuth)
	originalConfig := readFile(t, paths.CodexConfig)

	service := Service{
		Paths: paths,
		Registry: &registry.Registry{
			Version: registry.CurrentVersion,
			Providers: map[string]registry.Provider{
				"codex-only": {
					Codex: &registry.CodexProvider{APIKey: "new", BaseURL: "new"},
				},
			},
		},
	}
	if err := service.Switch(AgentAll, "codex-only"); err == nil {
		t.Fatal("Switch() succeeded for all with a codex-only provider")
	}
	if readFile(t, paths.CodexAuth) != originalAuth || readFile(t, paths.CodexConfig) != originalConfig {
		t.Fatal("Codex files changed after all-agent validation failed")
	}
}

func TestReplaceCodexBaseURLPreservesCRLFAndInsertsMissingField(t *testing.T) {
	input := "[model_providers.custom]\r\nname = \"Custom\"\r\n\r\n[tui]\r\nenabled = true\r\n"
	updated, err := replaceCodexBaseURL(input, "https://example.test/v1")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(strings.ReplaceAll(updated, "\r\n", ""), "\n") {
		t.Fatalf("updated config introduced LF line endings: %q", updated)
	}
	if !strings.Contains(updated, "base_url = \"https://example.test/v1\"\r\n[tui]") {
		t.Fatalf("base_url was not inserted before the next section: %q", updated)
	}
}

func TestSwitchClaudeCreatesMissingSettingsFile(t *testing.T) {
	paths := testPaths(t)
	service := Service{
		Paths: paths,
		Registry: &registry.Registry{
			Version: registry.CurrentVersion,
			Providers: map[string]registry.Provider{
				"relay": {
					Claude: &registry.ClaudeProvider{AuthToken: "token", BaseURL: "https://example.test"},
				},
			},
		},
	}
	if err := service.Switch(AgentClaude, "relay"); err != nil {
		t.Fatal(err)
	}
	settings := readJSON(t, paths.ClaudeSettings)
	env := settings["env"].(map[string]any)
	if env["ANTHROPIC_AUTH_TOKEN"] != "token" || env["ANTHROPIC_BASE_URL"] != "https://example.test" {
		t.Fatalf("created settings env = %#v", env)
	}
}

func testPaths(t *testing.T) configfile.Paths {
	t.Helper()
	home := t.TempDir()
	return configfile.Paths{
		Registry:       filepath.Join(home, ".agent-switch", "providers.yaml"),
		CodexAuth:      filepath.Join(home, ".codex", "auth.json"),
		CodexConfig:    filepath.Join(home, ".codex", "config.toml"),
		ClaudeSettings: filepath.Join(home, ".claude", "settings.json"),
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(content)
}

func readJSON(t *testing.T, path string) map[string]any {
	t.Helper()
	var value map[string]any
	if err := json.Unmarshal([]byte(readFile(t, path)), &value); err != nil {
		t.Fatal(err)
	}
	return value
}

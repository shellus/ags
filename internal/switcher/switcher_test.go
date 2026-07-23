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
		Defaults: registry.Provider{
			Codex:  &registry.CodexConfig{Model: "new-codex-model"},
			Claude: &registry.ClaudeConfig{Model: "new-claude-model"},
		},
		Providers: map[string]registry.Provider{
			"relay": {
				Codex:  &registry.CodexConfig{APIKey: "new-codex-key", BaseURL: "https://codex.example/v1"},
				Claude: &registry.ClaudeConfig{AuthToken: "new-claude-key", BaseURL: "https://claude.example"},
			},
			"same-connection": {
				Codex:  &registry.CodexConfig{APIKey: "new-codex-key", BaseURL: "https://codex.example/v1", Model: "other-codex-model"},
				Claude: &registry.ClaudeConfig{AuthToken: "new-claude-key", BaseURL: "https://claude.example", Model: "other-claude-model"},
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
	if !strings.Contains(config, `base_url = "https://keep.example/v1"`) || !strings.Contains(config, `model = "new-codex-model"`) || !strings.Contains(config, `model_provider = "custom"`) {
		t.Fatalf("Codex unrelated config changed:\n%s", config)
	}

	claude := readJSON(t, paths.ClaudeSettings)
	env := claude["env"].(map[string]any)
	if env["ANTHROPIC_AUTH_TOKEN"] != "new-claude-key" || env["ANTHROPIC_BASE_URL"] != "https://claude.example" {
		t.Fatalf("Claude provider fields = %#v", env)
	}
	if env["IS_SANDBOX"] != "1" || claude["model"] != "new-claude-model" {
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

func TestSwitchWithoutModelPreservesExistingModels(t *testing.T) {
	paths := testPaths(t)
	writeFile(t, paths.CodexAuth, `{"OPENAI_API_KEY":"old"}
`)
	writeFile(t, paths.CodexConfig, `model = "keep-codex-model"

[model_providers.custom]
base_url = "https://old.example/v1"
`)
	writeFile(t, paths.ClaudeSettings, `{"model":"keep-claude-model","env":{"ANTHROPIC_AUTH_TOKEN":"old","ANTHROPIC_BASE_URL":"https://old.example"}}
`)
	service := Service{
		Paths: paths,
		Registry: &registry.Registry{
			Version: registry.CurrentVersion,
			Providers: map[string]registry.Provider{
				"relay": {
					Codex:  &registry.CodexConfig{APIKey: "new", BaseURL: "https://codex.example/v1"},
					Claude: &registry.ClaudeConfig{AuthToken: "new", BaseURL: "https://claude.example"},
				},
			},
		},
	}

	if err := service.Switch(AgentAll, "relay"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(readFile(t, paths.CodexConfig), `model = "keep-codex-model"`) {
		t.Fatal("Codex model changed without a configured model")
	}
	if readJSON(t, paths.ClaudeSettings)["model"] != "keep-claude-model" {
		t.Fatal("Claude model changed without a configured model")
	}
	state, err := service.Current()
	if err != nil {
		t.Fatal(err)
	}
	if len(state.Codex) != 1 || state.Codex[0] != "relay" || len(state.Claude) != 1 || state.Claude[0] != "relay" {
		t.Fatalf("Current() = %#v", state)
	}
}

func TestSwitchWithoutModelDoesNotCreateModelFields(t *testing.T) {
	paths := testPaths(t)
	writeFile(t, paths.CodexAuth, `{"OPENAI_API_KEY":"old"}
`)
	writeFile(t, paths.CodexConfig, `[model_providers.custom]
base_url = "https://old.example/v1"
`)
	writeFile(t, paths.ClaudeSettings, `{"env":{"ANTHROPIC_AUTH_TOKEN":"old","ANTHROPIC_BASE_URL":"https://old.example"}}
`)
	service := Service{
		Paths: paths,
		Registry: &registry.Registry{
			Version: registry.CurrentVersion,
			Providers: map[string]registry.Provider{
				"relay": {
					Codex:  &registry.CodexConfig{APIKey: "new", BaseURL: "https://codex.example/v1"},
					Claude: &registry.ClaudeConfig{AuthToken: "new", BaseURL: "https://claude.example"},
				},
			},
		},
	}

	if err := service.Switch(AgentAll, "relay"); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(readFile(t, paths.CodexConfig), "model =") {
		t.Fatal("Codex model field was created without a configured model")
	}
	if _, ok := readJSON(t, paths.ClaudeSettings)["model"]; ok {
		t.Fatal("Claude model field was created without a configured model")
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
					Codex: &registry.CodexConfig{APIKey: "new", BaseURL: "new"},
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

func TestReplaceCodexModelPreservesCRLFAndOnlyUpdatesTopLevel(t *testing.T) {
	input := "model_provider = \"custom\"\r\nmodel_reasoning_effort = \"high\"\r\n\r\n[model_providers.custom]\r\nmodel = \"nested-model\"\r\nbase_url = \"https://old.example/v1\"\r\n"
	updated := replaceCodexModel(input, "new-model")
	if strings.Contains(strings.ReplaceAll(updated, "\r\n", ""), "\n") {
		t.Fatalf("updated config introduced LF line endings: %q", updated)
	}
	if !strings.Contains(updated, "model = \"new-model\"\r\n") {
		t.Fatalf("top-level model was not inserted: %q", updated)
	}
	if !strings.Contains(updated, "model = \"nested-model\"\r\n") || !strings.Contains(updated, "model_reasoning_effort = \"high\"\r\n") {
		t.Fatalf("non-target model fields changed: %q", updated)
	}
}

func TestReplaceCodexModelReplacesExistingTopLevelField(t *testing.T) {
	input := "model_provider = \"custom\"\nmodel = 'old-model'\n\n[model_providers.custom]\nbase_url = \"https://old.example/v1\"\n"
	updated := replaceCodexModel(input, "new-model")
	if strings.Count(updated, "model =") != 1 || !strings.Contains(updated, `model = "new-model"`) {
		t.Fatalf("top-level model was not replaced: %q", updated)
	}
	model, err := readCodexModel(updated)
	if err != nil {
		t.Fatal(err)
	}
	if model != "new-model" {
		t.Fatalf("readCodexModel() = %q, want new-model", model)
	}
}

func TestSwitchAllUsesUniversalConfigForBothAgents(t *testing.T) {
	paths := testPaths(t)
	writeFile(t, paths.CodexAuth, `{"OPENAI_API_KEY":"old"}
`)
	writeFile(t, paths.CodexConfig, `[model_providers.custom]
base_url = "https://old.example"
`)
	writeFile(t, paths.ClaudeSettings, `{"env":{"ANTHROPIC_AUTH_TOKEN":"old","ANTHROPIC_BASE_URL":"https://old.example"}}
`)

	service := Service{
		Paths: paths,
		Registry: &registry.Registry{
			Version: registry.CurrentVersion,
			Providers: map[string]registry.Provider{
				"shared": {
					Universal: &registry.UniversalConfig{APIKey: "shared-secret", BaseURL: "https://shared.example"},
				},
			},
		},
	}
	if err := service.Switch(AgentAll, "shared"); err != nil {
		t.Fatal(err)
	}

	auth := readJSON(t, paths.CodexAuth)
	if auth["OPENAI_API_KEY"] != "shared-secret" {
		t.Fatalf("Codex key = %#v", auth["OPENAI_API_KEY"])
	}
	if !strings.Contains(readFile(t, paths.CodexConfig), `base_url = "https://shared.example"`) {
		t.Fatalf("Codex config = %s", readFile(t, paths.CodexConfig))
	}
	claude := readJSON(t, paths.ClaudeSettings)
	env := claude["env"].(map[string]any)
	if env["ANTHROPIC_AUTH_TOKEN"] != "shared-secret" || env["ANTHROPIC_BASE_URL"] != "https://shared.example" {
		t.Fatalf("Claude env = %#v", env)
	}

	state, err := service.Current()
	if err != nil {
		t.Fatal(err)
	}
	if len(state.Codex) != 1 || state.Codex[0] != "shared" || len(state.Claude) != 1 || state.Claude[0] != "shared" {
		t.Fatalf("Current() = %#v", state)
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
					Claude: &registry.ClaudeConfig{AuthToken: "token", BaseURL: "https://example.test", Model: "claude-model"},
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
	if settings["model"] != "claude-model" {
		t.Fatalf("created settings model = %#v", settings["model"])
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

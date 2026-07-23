package app

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shellus/ags/internal/agent"
	"github.com/shellus/ags/internal/configfile"
	"github.com/shellus/ags/internal/registry"
	"github.com/shellus/ags/internal/switcher"
)

func TestListPrintsModelsAndBaseURLsWithoutSecrets(t *testing.T) {
	paths := appTestPaths(t)
	if err := os.MkdirAll(filepath.Dir(paths.Registry), 0o700); err != nil {
		t.Fatal(err)
	}
	content := `version: 2
defaults:
  codex:
    api_key: top-secret-codex
    model: codex-model
  claude:
    auth_token: top-secret-claude
    model: claude-model
providers:
  shared:
    universal:
      api_key: top-secret-shared
      base_url: https://shared.example
  relay:
    codex:
      base_url: https://codex.example/v1
    claude:
      base_url: https://claude.example
`
	if err := os.WriteFile(paths.Registry, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	var output bytes.Buffer
	runner := Runner{Paths: paths, Out: &output}
	if err := runner.Run([]string{"provider", "list"}); err != nil {
		t.Fatal(err)
	}
	result := output.String()
	if !strings.Contains(result, "relay") || !strings.Contains(result, "shared") || !strings.Contains(result, "CONFIG MODE") || !strings.Contains(result, "CODEX") || !strings.Contains(result, "CLAUDE") {
		t.Fatalf("list output = %q", result)
	}
	if !strings.Contains(result, "https://codex.example/v1") || !strings.Contains(result, "https://claude.example") {
		t.Fatalf("list output did not include base URLs: %q", result)
	}
	if !strings.Contains(result, "codex-model") || !strings.Contains(result, "claude-model") {
		t.Fatalf("list output did not include models: %q", result)
	}
	if strings.Count(result, "https://shared.example") != 2 || !strings.Contains(result, "universal") {
		t.Fatalf("list output did not show universal provider for both agents: %q", result)
	}
	if strings.Contains(result, "top-secret") {
		t.Fatalf("list output exposed a secret: %q", result)
	}
}

func TestListDisplaysDashForUnmanagedModels(t *testing.T) {
	paths := appTestPaths(t)
	writeAppFile(t, paths.Registry, `version: 2
providers:
  relay:
    codex:
      api_key: codex-key
      base_url: https://codex.example/v1
`)

	var output bytes.Buffer
	runner := Runner{Paths: paths, Out: &output}
	if err := runner.Run([]string{"provider", "list"}); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(output.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("list output = %q", output.String())
	}
	fields := strings.Fields(lines[1])
	if len(fields) != 6 || fields[0] != "relay" || fields[1] != "agent-specific" || fields[2] != "-" || fields[3] != "https://codex.example/v1" {
		t.Fatalf("list output = %q", output.String())
	}
}

func TestNoArgumentsSelectsAgentAndProvider(t *testing.T) {
	paths := writeAppFixture(t)
	selector := &fakeSelector{agent: switcher.AgentClaude, provider: "relay"}
	var output bytes.Buffer
	runner := Runner{Paths: paths, Out: &output, UI: selector, Interactive: true}

	if err := runner.Run(nil); err != nil {
		t.Fatal(err)
	}
	if selector.agentCalls != 1 || selector.providerAgent != switcher.AgentClaude {
		t.Fatalf("selector calls = %#v", selector)
	}
	if len(selector.current.Codex) != 1 || selector.current.Codex[0] != "relay" || len(selector.current.Claude) != 1 || selector.current.Claude[0] != "relay" {
		t.Fatalf("selector current state = %#v", selector.current)
	}
	settings := readAppJSON(t, paths.ClaudeSettings)
	env := settings["env"].(map[string]any)
	if env["ANTHROPIC_AUTH_TOKEN"] != "claude-secret" || env["ANTHROPIC_BASE_URL"] != "https://claude.example" {
		t.Fatalf("Claude env = %#v", env)
	}
	if settings["model"] != "claude-model" {
		t.Fatalf("Claude model = %#v", settings["model"])
	}
	if !strings.Contains(output.String(), "Switched claude provider to relay") {
		t.Fatalf("output = %q", output.String())
	}
	if !strings.Contains(output.String(), "codex: relay") || !strings.Contains(output.String(), "claude: relay") {
		t.Fatalf("current provider summary missing from output: %q", output.String())
	}
}

func TestAgentOnlySelectsProviderWithoutSelectingAgent(t *testing.T) {
	paths := writeAppFixture(t)
	selector := &fakeSelector{agent: switcher.AgentClaude, provider: "relay"}
	runner := Runner{Paths: paths, Out: &bytes.Buffer{}, UI: selector, Interactive: true}

	if err := runner.Run([]string{"provider", "switch", "codex"}); err != nil {
		t.Fatal(err)
	}
	if selector.agentCalls != 0 || selector.providerAgent != switcher.AgentCodex {
		t.Fatalf("selector calls = %#v", selector)
	}
	auth := readAppJSON(t, paths.CodexAuth)
	if auth["OPENAI_API_KEY"] != "codex-secret" {
		t.Fatalf("Codex key = %#v", auth["OPENAI_API_KEY"])
	}
	config := readAppFile(t, paths.CodexConfig)
	if !strings.Contains(config, `model = "codex-model"`) {
		t.Fatalf("Codex model was not updated: %q", config)
	}
}

type fakeSelector struct {
	agent         switcher.Agent
	provider      string
	agentCalls    int
	providerAgent switcher.Agent
	current       switcher.CurrentState
}

func (s *fakeSelector) SelectMainAction() (string, error) {
	return ActionProviderSwitch, nil
}

func (s *fakeSelector) SelectAgent() (switcher.Agent, error) {
	s.agentCalls++
	return s.agent, nil
}

func (s *fakeSelector) SelectProvider(agent switcher.Agent, _ *registry.Registry, current switcher.CurrentState) (string, error) {
	s.providerAgent = agent
	s.current = current
	return s.provider, nil
}

func (s *fakeSelector) SelectAgents(_ string, selected []agent.Name) ([]agent.Name, error) {
	return selected, nil
}

func (s *fakeSelector) ConfigureEnvironment(_ []string, profile string, agents []agent.Name) (string, []agent.Name, error) {
	return profile, agents, nil
}

func (s *fakeSelector) InputSource(source, branch string) (string, string, error) {
	return source, branch, nil
}

func (s *fakeSelector) Confirm(string) (bool, error) {
	return true, nil
}

func appTestPaths(t *testing.T) configfile.Paths {
	t.Helper()
	home := t.TempDir()
	return configfile.Paths{
		ConfigDir:      filepath.Join(home, ".config", "ags"),
		CacheDir:       filepath.Join(home, ".cache", "ags"),
		StateDir:       filepath.Join(home, ".local", "state", "ags"),
		AGSConfig:      filepath.Join(home, ".config", "ags", "config.yaml"),
		Registry:       filepath.Join(home, ".config", "ags", "providers.yaml"),
		CodexDir:       filepath.Join(home, ".codex"),
		CodexAuth:      filepath.Join(home, ".codex", "auth.json"),
		CodexConfig:    filepath.Join(home, ".codex", "config.toml"),
		CodexGuidance:  filepath.Join(home, ".codex", "AGENTS.md"),
		CodexSkills:    filepath.Join(home, ".codex", "skills"),
		ClaudeDir:      filepath.Join(home, ".claude"),
		ClaudeSettings: filepath.Join(home, ".claude", "settings.json"),
		ClaudeGuidance: filepath.Join(home, ".claude", "CLAUDE.md"),
		ClaudeSkills:   filepath.Join(home, ".claude", "skills"),
	}
}

func writeAppFixture(t *testing.T) configfile.Paths {
	t.Helper()
	paths := appTestPaths(t)
	writeAppFile(t, paths.Registry, `version: 2
defaults:
  codex:
    model: codex-model
  claude:
    model: claude-model
providers:
  relay:
    codex:
      api_key: codex-secret
      base_url: https://codex.example/v1
    claude:
      auth_token: claude-secret
      base_url: https://claude.example
`)
	writeAppFile(t, paths.CodexAuth, `{"OPENAI_API_KEY":"codex-secret"}
`)
	writeAppFile(t, paths.CodexConfig, `model = "codex-model"

[model_providers.custom]
base_url = "https://codex.example/v1"
`)
	writeAppFile(t, paths.ClaudeSettings, `{"model":"claude-model","env":{"ANTHROPIC_AUTH_TOKEN":"claude-secret","ANTHROPIC_BASE_URL":"https://claude.example"}}
`)
	return paths
}

func writeAppFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func readAppJSON(t *testing.T, path string) map[string]any {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var value map[string]any
	if err := json.Unmarshal(content, &value); err != nil {
		t.Fatal(err)
	}
	return value
}

func readAppFile(t *testing.T, path string) string {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(content)
}

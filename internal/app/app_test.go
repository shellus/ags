package app

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shellus/ags/internal/configfile"
	"github.com/shellus/ags/internal/registry"
	"github.com/shellus/ags/internal/switcher"
)

func TestListPrintsBaseURLsWithoutSecrets(t *testing.T) {
	paths := appTestPaths(t)
	if err := os.MkdirAll(filepath.Dir(paths.Registry), 0o700); err != nil {
		t.Fatal(err)
	}
	content := `version: 1
providers:
  shared:
    universal:
      api_key: top-secret-shared
      base_url: https://shared.example
  relay:
    codex:
      api_key: top-secret-codex
      base_url: https://codex.example/v1
    claude:
      auth_token: top-secret-claude
      base_url: https://claude.example
`
	if err := os.WriteFile(paths.Registry, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	var output bytes.Buffer
	runner := Runner{Paths: paths, Out: &output}
	if err := runner.Run([]string{"list"}); err != nil {
		t.Fatal(err)
	}
	result := output.String()
	if !strings.Contains(result, "relay") || !strings.Contains(result, "shared") || !strings.Contains(result, "CONFIG MODE") || !strings.Contains(result, "CODEX") || !strings.Contains(result, "CLAUDE") {
		t.Fatalf("list output = %q", result)
	}
	if !strings.Contains(result, "https://codex.example/v1") || !strings.Contains(result, "https://claude.example") {
		t.Fatalf("list output did not include base URLs: %q", result)
	}
	if strings.Count(result, "https://shared.example") != 2 || !strings.Contains(result, "universal") {
		t.Fatalf("list output did not show universal provider for both agents: %q", result)
	}
	if strings.Contains(result, "top-secret") {
		t.Fatalf("list output exposed a secret: %q", result)
	}
}

func TestNoArgumentsSelectsAgentAndProvider(t *testing.T) {
	paths := writeAppFixture(t)
	selector := &fakeSelector{agent: switcher.AgentClaude, provider: "relay"}
	var output bytes.Buffer
	runner := Runner{Paths: paths, Out: &output, Selector: selector}

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
	runner := Runner{Paths: paths, Out: &bytes.Buffer{}, Selector: selector}

	if err := runner.Run([]string{"codex"}); err != nil {
		t.Fatal(err)
	}
	if selector.agentCalls != 0 || selector.providerAgent != switcher.AgentCodex {
		t.Fatalf("selector calls = %#v", selector)
	}
	auth := readAppJSON(t, paths.CodexAuth)
	if auth["OPENAI_API_KEY"] != "codex-secret" {
		t.Fatalf("Codex key = %#v", auth["OPENAI_API_KEY"])
	}
}

type fakeSelector struct {
	agent         switcher.Agent
	provider      string
	agentCalls    int
	providerAgent switcher.Agent
	current       switcher.CurrentState
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

func appTestPaths(t *testing.T) configfile.Paths {
	t.Helper()
	home := t.TempDir()
	return configfile.Paths{
		Registry:       filepath.Join(home, ".agent-switch", "providers.yaml"),
		CodexAuth:      filepath.Join(home, ".codex", "auth.json"),
		CodexConfig:    filepath.Join(home, ".codex", "config.toml"),
		ClaudeSettings: filepath.Join(home, ".claude", "settings.json"),
	}
}

func writeAppFixture(t *testing.T) configfile.Paths {
	t.Helper()
	paths := appTestPaths(t)
	writeAppFile(t, paths.Registry, `version: 1
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
	writeAppFile(t, paths.CodexConfig, `[model_providers.custom]
base_url = "https://codex.example/v1"
`)
	writeAppFile(t, paths.ClaudeSettings, `{"env":{"ANTHROPIC_AUTH_TOKEN":"claude-secret","ANTHROPIC_BASE_URL":"https://claude.example"}}
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

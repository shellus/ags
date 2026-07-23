package interactive

import (
	"strings"
	"testing"

	"github.com/shellus/ags/internal/registry"
	"github.com/shellus/ags/internal/switcher"
)

func TestProviderLabelIncludesNameModelsBaseURLsAndCurrentMarker(t *testing.T) {
	provider := registry.Provider{
		Codex:  &registry.CodexConfig{BaseURL: "https://codex.example/v1", Model: "codex-model"},
		Claude: &registry.ClaudeConfig{BaseURL: "https://claude.example", Model: "claude-model"},
	}
	current := switcher.CurrentState{Codex: []string{"relay"}, Claude: []string{"relay"}}

	codexLabel, ok := providerLabel(switcher.AgentCodex, "relay", provider, registry.ConfigModeAgentSpecific, current)
	if !ok || !strings.Contains(codexLabel, "relay") || !strings.Contains(codexLabel, "codex-model") || !strings.Contains(codexLabel, "https://codex.example/v1") || !strings.Contains(codexLabel, "[current]") {
		t.Fatalf("Codex label = %q, %v", codexLabel, ok)
	}
	claudeLabel, ok := providerLabel(switcher.AgentClaude, "relay", provider, registry.ConfigModeAgentSpecific, current)
	if !ok || !strings.Contains(claudeLabel, "relay") || !strings.Contains(claudeLabel, "claude-model") || !strings.Contains(claudeLabel, "https://claude.example") || !strings.Contains(claudeLabel, "[current]") {
		t.Fatalf("Claude label = %q, %v", claudeLabel, ok)
	}
	allLabel, ok := providerLabel(switcher.AgentAll, "relay", provider, registry.ConfigModeAgentSpecific, current)
	if !ok || !strings.Contains(allLabel, "codex-model") || !strings.Contains(allLabel, "https://codex.example/v1") || !strings.Contains(allLabel, "claude-model") || !strings.Contains(allLabel, "https://claude.example") || !strings.Contains(allLabel, "[current]") {
		t.Fatalf("All label = %q, %v", allLabel, ok)
	}
}

func TestProviderOptionsFilterUnsupportedAgents(t *testing.T) {
	providerRegistry := &registry.Registry{
		Providers: map[string]registry.Provider{
			"both": {
				Codex:  &registry.CodexConfig{BaseURL: "codex-both", Model: "codex-model"},
				Claude: &registry.ClaudeConfig{BaseURL: "claude-both", Model: "claude-model"},
			},
			"codex-only": {
				Codex: &registry.CodexConfig{BaseURL: "codex-only", Model: "codex-model"},
			},
		},
	}

	current := switcher.CurrentState{Codex: []string{"both"}, Claude: []string{"both"}}
	if got := len(providerOptions(switcher.AgentCodex, providerRegistry, current)); got != 2 {
		t.Fatalf("Codex options = %d, want 2", got)
	}
	if got := len(providerOptions(switcher.AgentClaude, providerRegistry, current)); got != 1 {
		t.Fatalf("Claude options = %d, want 1", got)
	}
	if got := len(providerOptions(switcher.AgentAll, providerRegistry, current)); got != 1 {
		t.Fatalf("All options = %d, want 1", got)
	}
	if got := defaultProviderName(switcher.AgentCodex, providerRegistry, current); got != "both" {
		t.Fatalf("Codex default provider = %q, want both", got)
	}
	if got := defaultProviderName(switcher.AgentClaude, providerRegistry, current); got != "both" {
		t.Fatalf("Claude default provider = %q, want both", got)
	}
	if got := defaultProviderName(switcher.AgentAll, providerRegistry, current); got != "both" {
		t.Fatalf("All default provider = %q, want both", got)
	}
}

func TestUniversalProviderAppearsForEveryAgent(t *testing.T) {
	providerRegistry := &registry.Registry{
		Version: registry.CurrentVersion,
		Defaults: registry.Provider{
			Codex:  &registry.CodexConfig{Model: "codex-default"},
			Claude: &registry.ClaudeConfig{Model: "claude-default"},
		},
		Providers: map[string]registry.Provider{
			"shared": {
				Universal: &registry.UniversalConfig{APIKey: "shared", BaseURL: "https://shared.example"},
			},
		},
	}
	provider, err := providerRegistry.Provider("shared")
	if err != nil {
		t.Fatal(err)
	}
	current := switcher.CurrentState{Codex: []string{"shared"}, Claude: []string{"shared"}}

	for _, agent := range []switcher.Agent{switcher.AgentCodex, switcher.AgentClaude, switcher.AgentAll} {
		label, ok := providerLabel(agent, "shared", provider, registry.ConfigModeUniversal, current)
		if !ok || !strings.Contains(label, "shared") || !strings.Contains(label, "https://shared.example") || !strings.Contains(label, "[current]") {
			t.Fatalf("providerLabel(%s) = %q, %v", agent, label, ok)
		}
	}
	allLabel, _ := providerLabel(switcher.AgentAll, "shared", provider, registry.ConfigModeUniversal, current)
	if !strings.Contains(allLabel, "Universal:") || !strings.Contains(allLabel, "codex-default") || !strings.Contains(allLabel, "claude-default") {
		t.Fatalf("All label = %q", allLabel)
	}
}

func TestProviderLabelDisplaysDashWhenModelIsUnmanaged(t *testing.T) {
	provider := registry.Provider{
		Codex: &registry.CodexConfig{BaseURL: "https://codex.example/v1"},
	}
	label, ok := providerLabel(switcher.AgentCodex, "relay", provider, registry.ConfigModeAgentSpecific, switcher.CurrentState{})
	if !ok || !strings.Contains(label, "relay  -  https://codex.example/v1") {
		t.Fatalf("Codex label = %q, %v", label, ok)
	}
}

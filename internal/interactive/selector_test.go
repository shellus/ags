package interactive

import (
	"strings"
	"testing"

	"github.com/shellus/ags/internal/registry"
	"github.com/shellus/ags/internal/switcher"
)

func TestProviderLabelIncludesNameAndBaseURL(t *testing.T) {
	provider := registry.Provider{
		Codex:  &registry.CodexConfig{BaseURL: "https://codex.example/v1"},
		Claude: &registry.ClaudeConfig{BaseURL: "https://claude.example"},
	}

	current := switcher.CurrentState{Codex: []string{"relay"}, Claude: []string{"relay"}}
	codexLabel, ok := providerLabel(switcher.AgentCodex, "relay", provider, current)
	if !ok || !strings.Contains(codexLabel, "relay") || !strings.Contains(codexLabel, "https://codex.example/v1") {
		t.Fatalf("Codex label = %q, %v", codexLabel, ok)
	}
	if !strings.Contains(codexLabel, "[current]") {
		t.Fatalf("Codex label does not mark current provider: %q", codexLabel)
	}
	claudeLabel, ok := providerLabel(switcher.AgentClaude, "relay", provider, current)
	if !ok || !strings.Contains(claudeLabel, "relay") || !strings.Contains(claudeLabel, "https://claude.example") {
		t.Fatalf("Claude label = %q, %v", claudeLabel, ok)
	}
	allLabel, ok := providerLabel(switcher.AgentAll, "relay", provider, current)
	if !ok || !strings.Contains(allLabel, "https://codex.example/v1") || !strings.Contains(allLabel, "https://claude.example") {
		t.Fatalf("All label = %q, %v", allLabel, ok)
	}
}

func TestProviderOptionsFilterUnsupportedAgents(t *testing.T) {
	providerRegistry := &registry.Registry{
		Providers: map[string]registry.Provider{
			"both": {
				Codex:  &registry.CodexConfig{BaseURL: "codex-both"},
				Claude: &registry.ClaudeConfig{BaseURL: "claude-both"},
			},
			"codex-only": {
				Codex: &registry.CodexConfig{BaseURL: "codex-only"},
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
	provider := registry.Provider{
		Universal: &registry.UniversalConfig{BaseURL: "https://shared.example"},
	}
	current := switcher.CurrentState{Codex: []string{"shared"}, Claude: []string{"shared"}}

	for _, agent := range []switcher.Agent{switcher.AgentCodex, switcher.AgentClaude, switcher.AgentAll} {
		label, ok := providerLabel(agent, "shared", provider, current)
		if !ok || !strings.Contains(label, "shared") || !strings.Contains(label, "https://shared.example") || !strings.Contains(label, "[current]") {
			t.Fatalf("providerLabel(%s) = %q, %v", agent, label, ok)
		}
	}
	allLabel, _ := providerLabel(switcher.AgentAll, "shared", provider, current)
	if !strings.Contains(allLabel, "Universal:") {
		t.Fatalf("All label = %q, want Universal label", allLabel)
	}
}

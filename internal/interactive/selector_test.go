package interactive

import (
	"strings"
	"testing"

	"github.com/shellus/ags/internal/registry"
	"github.com/shellus/ags/internal/switcher"
)

func TestProviderLabelIncludesNameAndBaseURL(t *testing.T) {
	provider := registry.Provider{
		Codex:  &registry.CodexProvider{BaseURL: "https://codex.example/v1"},
		Claude: &registry.ClaudeProvider{BaseURL: "https://claude.example"},
	}

	codexLabel, ok := providerLabel(switcher.AgentCodex, "relay", provider)
	if !ok || !strings.Contains(codexLabel, "relay") || !strings.Contains(codexLabel, "https://codex.example/v1") {
		t.Fatalf("Codex label = %q, %v", codexLabel, ok)
	}
	claudeLabel, ok := providerLabel(switcher.AgentClaude, "relay", provider)
	if !ok || !strings.Contains(claudeLabel, "relay") || !strings.Contains(claudeLabel, "https://claude.example") {
		t.Fatalf("Claude label = %q, %v", claudeLabel, ok)
	}
	allLabel, ok := providerLabel(switcher.AgentAll, "relay", provider)
	if !ok || !strings.Contains(allLabel, "https://codex.example/v1") || !strings.Contains(allLabel, "https://claude.example") {
		t.Fatalf("All label = %q, %v", allLabel, ok)
	}
}

func TestProviderOptionsFilterUnsupportedAgents(t *testing.T) {
	providerRegistry := &registry.Registry{
		Providers: map[string]registry.Provider{
			"both": {
				Codex:  &registry.CodexProvider{BaseURL: "codex-both"},
				Claude: &registry.ClaudeProvider{BaseURL: "claude-both"},
			},
			"codex-only": {
				Codex: &registry.CodexProvider{BaseURL: "codex-only"},
			},
		},
	}

	if got := len(providerOptions(switcher.AgentCodex, providerRegistry)); got != 2 {
		t.Fatalf("Codex options = %d, want 2", got)
	}
	if got := len(providerOptions(switcher.AgentClaude, providerRegistry)); got != 1 {
		t.Fatalf("Claude options = %d, want 1", got)
	}
	if got := len(providerOptions(switcher.AgentAll, providerRegistry)); got != 1 {
		t.Fatalf("All options = %d, want 1", got)
	}
}

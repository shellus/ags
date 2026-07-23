package registry

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadValidRegistry(t *testing.T) {
	path := writeRegistry(t, `version: 2
providers:
  relay:
    codex:
      api_key: codex-secret
      base_url: https://codex.example/v1
      model: codex-model
    claude:
      auth_token: claude-secret
      base_url: https://claude.example
      model: claude-model
`)

	loaded, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	provider, err := loaded.Provider("relay")
	if err != nil {
		t.Fatal(err)
	}
	if provider.Codex == nil || provider.Claude == nil {
		t.Fatalf("provider adapters were not loaded: %#v", provider)
	}
	if provider.Codex.Model != "codex-model" || provider.Claude.Model != "claude-model" {
		t.Fatalf("provider models were not loaded: %#v", provider)
	}
}

func TestLoadRejectsUnknownFields(t *testing.T) {
	path := writeRegistry(t, `version: 2
providers:
  relay:
    codex:
      api_key: secret
      base_url: https://example.test/v1
      model: example-model
      unexpected: true
`)

	_, err := Load(path)
	if err == nil || !strings.Contains(err.Error(), "field unexpected not found") {
		t.Fatalf("Load() error = %v, want unknown field error", err)
	}
}

func TestLoadRejectsVersionOneRegistry(t *testing.T) {
	path := writeRegistry(t, `version: 1
providers:
  relay:
    codex:
      api_key: secret
      base_url: https://example.test/v1
      model: example-model
`)

	_, err := Load(path)
	if err == nil || !strings.Contains(err.Error(), "unsupported version 1, expected 2") {
		t.Fatalf("Load() error = %v, want version migration error", err)
	}
}

func TestLoadAppliesDefaultsAndProviderOverrides(t *testing.T) {
	path := writeRegistry(t, `version: 2
defaults:
  codex:
    api_key: default-codex-key
    base_url: https://default-codex.example/v1
    model: default-codex-model
  claude:
    auth_token: default-claude-token
    base_url: https://default-claude.example
    model: default-claude-model
providers:
  relay:
    codex:
      base_url: https://relay-codex.example/v1
    claude:
      model: relay-claude-model
`)

	loaded, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	provider, err := loaded.Provider("relay")
	if err != nil {
		t.Fatal(err)
	}
	if provider.Codex == nil || provider.Codex.APIKey != "default-codex-key" || provider.Codex.BaseURL != "https://relay-codex.example/v1" || provider.Codex.Model != "default-codex-model" {
		t.Fatalf("resolved Codex provider = %#v", provider.Codex)
	}
	if provider.Claude == nil || provider.Claude.AuthToken != "default-claude-token" || provider.Claude.BaseURL != "https://default-claude.example" || provider.Claude.Model != "relay-claude-model" {
		t.Fatalf("resolved Claude provider = %#v", provider.Claude)
	}
}

func TestValidateAllowsMissingModels(t *testing.T) {
	providerRegistry := Registry{
		Version: CurrentVersion,
		Providers: map[string]Provider{
			"relay": {
				Codex:  &CodexConfig{APIKey: "key", BaseURL: "https://codex.example.test/v1"},
				Claude: &ClaudeConfig{AuthToken: "token", BaseURL: "https://claude.example.test"},
			},
		},
	}
	if err := providerRegistry.Validate(); err != nil {
		t.Fatal(err)
	}
	provider, err := providerRegistry.Provider("relay")
	if err != nil {
		t.Fatal(err)
	}
	if provider.Codex.Model != "" || provider.Claude.Model != "" {
		t.Fatalf("resolved optional models = %#v", provider)
	}
}

func TestValidateRequiresEffectiveConnectionFields(t *testing.T) {
	tests := []struct {
		name     string
		provider Provider
		want     string
	}{
		{
			name: "codex api key",
			provider: Provider{
				Codex: &CodexConfig{BaseURL: "https://example.test/v1"},
			},
			want: "codex.api_key must not be empty",
		},
		{
			name: "claude base URL",
			provider: Provider{
				Claude: &ClaudeConfig{AuthToken: "token"},
			},
			want: "claude.base_url must not be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			providerRegistry := Registry{
				Version: CurrentVersion,
				Providers: map[string]Provider{
					"relay": tt.provider,
				},
			}
			err := providerRegistry.Validate()
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("Validate() error = %v, want %q", err, tt.want)
			}
		})
	}
}

func TestValidateRequiresAgentConfiguration(t *testing.T) {
	registry := Registry{
		Version: CurrentVersion,
		Providers: map[string]Provider{
			"empty": {},
		},
	}
	if err := registry.Validate(); err == nil {
		t.Fatal("Validate() succeeded for provider without agent configuration")
	}
}

func TestUniversalConfigSupportsBothAgents(t *testing.T) {
	provider := Provider{
		Universal: &UniversalConfig{APIKey: "shared-secret", BaseURL: "https://shared.example"},
	}

	codex, ok := provider.EffectiveCodex()
	if !ok || codex.APIKey != "shared-secret" || codex.BaseURL != "https://shared.example" {
		t.Fatalf("EffectiveCodex() = %#v, %v", codex, ok)
	}
	claude, ok := provider.EffectiveClaude()
	if !ok || claude.AuthToken != "shared-secret" || claude.BaseURL != "https://shared.example" {
		t.Fatalf("EffectiveClaude() = %#v, %v", claude, ok)
	}
	if provider.ConfigMode() != ConfigModeUniversal {
		t.Fatalf("ConfigMode() = %q, want universal", provider.ConfigMode())
	}
}

func TestAgentSpecificConfigOverridesUniversal(t *testing.T) {
	provider := Provider{
		Universal: &UniversalConfig{APIKey: "shared-secret", BaseURL: "https://shared.example"},
		Codex:     &CodexConfig{APIKey: "codex-secret", BaseURL: "https://codex.example/v1"},
	}

	codex, ok := provider.EffectiveCodex()
	if !ok || codex.APIKey != "codex-secret" || codex.BaseURL != "https://codex.example/v1" {
		t.Fatalf("EffectiveCodex() = %#v, %v", codex, ok)
	}
	claude, ok := provider.EffectiveClaude()
	if !ok || claude.AuthToken != "shared-secret" || claude.BaseURL != "https://shared.example" {
		t.Fatalf("EffectiveClaude() = %#v, %v", claude, ok)
	}
	if provider.ConfigMode() != ConfigModeMixed {
		t.Fatalf("ConfigMode() = %q, want mixed", provider.ConfigMode())
	}
}

func TestRegistryProviderAppliesAgentDefaultsToUniversalConfig(t *testing.T) {
	providerRegistry := Registry{
		Version: CurrentVersion,
		Defaults: Provider{
			Codex:  &CodexConfig{Model: "codex-default"},
			Claude: &ClaudeConfig{Model: "claude-default"},
		},
		Providers: map[string]Provider{
			"shared": {
				Universal: &UniversalConfig{APIKey: "shared-secret", BaseURL: "https://shared.example"},
			},
		},
	}
	if err := providerRegistry.Validate(); err != nil {
		t.Fatal(err)
	}
	provider, err := providerRegistry.Provider("shared")
	if err != nil {
		t.Fatal(err)
	}
	if provider.Codex == nil || provider.Codex.APIKey != "shared-secret" || provider.Codex.Model != "codex-default" {
		t.Fatalf("resolved Codex provider = %#v", provider.Codex)
	}
	if provider.Claude == nil || provider.Claude.AuthToken != "shared-secret" || provider.Claude.Model != "claude-default" {
		t.Fatalf("resolved Claude provider = %#v", provider.Claude)
	}
}

func TestAgentSpecificConfigDoesNotInheritUniversalFields(t *testing.T) {
	providerRegistry := Registry{
		Version: CurrentVersion,
		Providers: map[string]Provider{
			"mixed": {
				Universal: &UniversalConfig{APIKey: "shared-secret", BaseURL: "https://shared.example"},
				Codex:     &CodexConfig{Model: "codex-model"},
			},
		},
	}
	err := providerRegistry.Validate()
	if err == nil || !strings.Contains(err.Error(), "codex.api_key must not be empty") {
		t.Fatalf("Validate() error = %v, want agent-specific connection error", err)
	}
}

func TestValidateRejectsIncompleteUniversalConfig(t *testing.T) {
	providerRegistry := Registry{
		Version: CurrentVersion,
		Providers: map[string]Provider{
			"shared": {Universal: &UniversalConfig{BaseURL: "https://shared.example"}},
		},
	}
	if err := providerRegistry.Validate(); err == nil || !strings.Contains(err.Error(), "universal.api_key") {
		t.Fatalf("Validate() error = %v, want universal.api_key error", err)
	}
}

func writeRegistry(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "providers.yaml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

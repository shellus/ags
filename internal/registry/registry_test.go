package registry

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadValidRegistry(t *testing.T) {
	path := writeRegistry(t, `version: 1
providers:
  relay:
    codex:
      api_key: codex-secret
      base_url: https://codex.example/v1
    claude:
      auth_token: claude-secret
      base_url: https://claude.example
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
}

func TestLoadRejectsUnknownFields(t *testing.T) {
	path := writeRegistry(t, `version: 1
providers:
  relay:
    codex:
      api_key: secret
      base_url: https://example.test/v1
      unexpected: true
`)

	_, err := Load(path)
	if err == nil || !strings.Contains(err.Error(), "field unexpected not found") {
		t.Fatalf("Load() error = %v, want unknown field error", err)
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

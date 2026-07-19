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

func writeRegistry(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "providers.yaml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

package registry

import (
	"bytes"
	"fmt"
	"os"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

const CurrentVersion = 2

type ConfigMode string

const (
	ConfigModeAgentSpecific ConfigMode = "agent-specific"
	ConfigModeMixed         ConfigMode = "mixed"
	ConfigModeUniversal     ConfigMode = "universal"
)

type Registry struct {
	Version   int                 `yaml:"version"`
	Defaults  Provider            `yaml:"defaults,omitempty"`
	Providers map[string]Provider `yaml:"providers"`
}

type Provider struct {
	Universal *UniversalConfig `yaml:"universal,omitempty"`
	Codex     *CodexConfig     `yaml:"codex,omitempty"`
	Claude    *ClaudeConfig    `yaml:"claude,omitempty"`
}

type UniversalConfig struct {
	APIKey  string `yaml:"api_key"`
	BaseURL string `yaml:"base_url"`
}

type CodexConfig struct {
	APIKey  string `yaml:"api_key"`
	BaseURL string `yaml:"base_url"`
	Model   string `yaml:"model"`
}

type ClaudeConfig struct {
	AuthToken string `yaml:"auth_token"`
	BaseURL   string `yaml:"base_url"`
	Model     string `yaml:"model"`
}

func (p Provider) EffectiveCodex() (CodexConfig, bool) {
	if p.Codex != nil {
		return *p.Codex, true
	}
	if p.Universal != nil {
		return CodexConfig{APIKey: p.Universal.APIKey, BaseURL: p.Universal.BaseURL}, true
	}
	return CodexConfig{}, false
}

func (p Provider) EffectiveClaude() (ClaudeConfig, bool) {
	if p.Claude != nil {
		return *p.Claude, true
	}
	if p.Universal != nil {
		return ClaudeConfig{AuthToken: p.Universal.APIKey, BaseURL: p.Universal.BaseURL}, true
	}
	return ClaudeConfig{}, false
}

func (p Provider) ConfigMode() ConfigMode {
	if p.Universal == nil {
		return ConfigModeAgentSpecific
	}
	if p.Codex != nil || p.Claude != nil {
		return ConfigModeMixed
	}
	return ConfigModeUniversal
}

func Load(path string) (*Registry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read provider registry %s: %w", path, err)
	}

	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)

	var providerRegistry Registry
	if err := decoder.Decode(&providerRegistry); err != nil {
		return nil, fmt.Errorf("parse provider registry %s: %w", path, err)
	}
	if err := providerRegistry.Validate(); err != nil {
		return nil, fmt.Errorf("validate provider registry %s: %w", path, err)
	}
	return &providerRegistry, nil
}

func (r *Registry) Validate() error {
	if r.Version != CurrentVersion {
		return fmt.Errorf("unsupported version %d, expected %d", r.Version, CurrentVersion)
	}
	if len(r.Providers) == 0 {
		return fmt.Errorf("providers must contain at least one entry")
	}

	for name, provider := range r.Providers {
		if strings.TrimSpace(name) == "" {
			return fmt.Errorf("provider name must not be empty")
		}
		if provider.Universal == nil && provider.Codex == nil && provider.Claude == nil {
			return fmt.Errorf("provider %q must configure universal, codex, claude, or a combination", name)
		}
		if provider.Universal != nil {
			if strings.TrimSpace(provider.Universal.APIKey) == "" {
				return fmt.Errorf("provider %q universal.api_key must not be empty", name)
			}
			if strings.TrimSpace(provider.Universal.BaseURL) == "" {
				return fmt.Errorf("provider %q universal.base_url must not be empty", name)
			}
		}

		resolved := r.resolveProvider(provider)
		if resolved.Codex != nil {
			if strings.TrimSpace(resolved.Codex.APIKey) == "" {
				return fmt.Errorf("provider %q codex.api_key must not be empty", name)
			}
			if strings.TrimSpace(resolved.Codex.BaseURL) == "" {
				return fmt.Errorf("provider %q codex.base_url must not be empty", name)
			}
		}
		if resolved.Claude != nil {
			if strings.TrimSpace(resolved.Claude.AuthToken) == "" {
				return fmt.Errorf("provider %q claude.auth_token must not be empty", name)
			}
			if strings.TrimSpace(resolved.Claude.BaseURL) == "" {
				return fmt.Errorf("provider %q claude.base_url must not be empty", name)
			}
		}
	}
	return nil
}

func (r *Registry) Provider(name string) (Provider, error) {
	provider, ok := r.Providers[name]
	if !ok {
		return Provider{}, fmt.Errorf("unknown provider %q; available providers: %s", name, strings.Join(r.Names(), ", "))
	}
	return r.resolveProvider(provider), nil
}

func (r *Registry) resolveProvider(provider Provider) Provider {
	codex, supportsCodex := provider.EffectiveCodex()
	claude, supportsClaude := provider.EffectiveClaude()

	resolved := Provider{}
	if supportsCodex {
		resolved.Codex = mergeCodexConfig(r.Defaults.Codex, &codex)
	}
	if supportsClaude {
		resolved.Claude = mergeClaudeConfig(r.Defaults.Claude, &claude)
	}
	return resolved
}

func mergeCodexConfig(defaults, provider *CodexConfig) *CodexConfig {
	if provider == nil {
		return nil
	}
	defaultValues := CodexConfig{}
	if defaults != nil {
		defaultValues = *defaults
	}
	return &CodexConfig{
		APIKey:  mergedValue(provider.APIKey, defaultValues.APIKey),
		BaseURL: mergedValue(provider.BaseURL, defaultValues.BaseURL),
		Model:   mergedValue(provider.Model, defaultValues.Model),
	}
}

func mergeClaudeConfig(defaults, provider *ClaudeConfig) *ClaudeConfig {
	if provider == nil {
		return nil
	}
	defaultValues := ClaudeConfig{}
	if defaults != nil {
		defaultValues = *defaults
	}
	return &ClaudeConfig{
		AuthToken: mergedValue(provider.AuthToken, defaultValues.AuthToken),
		BaseURL:   mergedValue(provider.BaseURL, defaultValues.BaseURL),
		Model:     mergedValue(provider.Model, defaultValues.Model),
	}
}

func mergedValue(providerValue, defaultValue string) string {
	if strings.TrimSpace(providerValue) != "" {
		return providerValue
	}
	if strings.TrimSpace(defaultValue) != "" {
		return defaultValue
	}
	return ""
}

func (r *Registry) Names() []string {
	names := make([]string, 0, len(r.Providers))
	for name := range r.Providers {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

package registry

import (
	"bytes"
	"fmt"
	"os"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

const CurrentVersion = 1

type Registry struct {
	Version   int                 `yaml:"version"`
	Providers map[string]Provider `yaml:"providers"`
}

type Provider struct {
	Codex  *CodexProvider  `yaml:"codex,omitempty"`
	Claude *ClaudeProvider `yaml:"claude,omitempty"`
}

type CodexProvider struct {
	APIKey  string `yaml:"api_key"`
	BaseURL string `yaml:"base_url"`
}

type ClaudeProvider struct {
	AuthToken string `yaml:"auth_token"`
	BaseURL   string `yaml:"base_url"`
}

func Load(path string) (*Registry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read provider registry %s: %w", path, err)
	}

	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)

	var registry Registry
	if err := decoder.Decode(&registry); err != nil {
		return nil, fmt.Errorf("parse provider registry %s: %w", path, err)
	}
	if err := registry.Validate(); err != nil {
		return nil, fmt.Errorf("validate provider registry %s: %w", path, err)
	}
	return &registry, nil
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
		if provider.Codex == nil && provider.Claude == nil {
			return fmt.Errorf("provider %q must configure codex, claude, or both", name)
		}
		if provider.Codex != nil {
			if strings.TrimSpace(provider.Codex.APIKey) == "" {
				return fmt.Errorf("provider %q codex.api_key must not be empty", name)
			}
			if strings.TrimSpace(provider.Codex.BaseURL) == "" {
				return fmt.Errorf("provider %q codex.base_url must not be empty", name)
			}
		}
		if provider.Claude != nil {
			if strings.TrimSpace(provider.Claude.AuthToken) == "" {
				return fmt.Errorf("provider %q claude.auth_token must not be empty", name)
			}
			if strings.TrimSpace(provider.Claude.BaseURL) == "" {
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
	return provider, nil
}

func (r *Registry) Names() []string {
	names := make([]string, 0, len(r.Providers))
	for name := range r.Providers {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

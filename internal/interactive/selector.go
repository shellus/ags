package interactive

import (
	"fmt"

	"charm.land/huh/v2"
	"github.com/shellus/ags/internal/registry"
	"github.com/shellus/ags/internal/switcher"
)

type Selector struct{}

func (Selector) SelectAgent() (switcher.Agent, error) {
	var agent switcher.Agent
	err := huh.NewSelect[switcher.Agent]().
		Title("选择 Agent").
		Options(
			huh.NewOption("Codex", switcher.AgentCodex),
			huh.NewOption("Claude", switcher.AgentClaude),
			huh.NewOption("All (Codex + Claude)", switcher.AgentAll),
		).
		Value(&agent).
		Run()
	return agent, err
}

func (Selector) SelectProvider(agent switcher.Agent, providerRegistry *registry.Registry, current switcher.CurrentState) (string, error) {
	options := providerOptions(agent, providerRegistry, current)
	if len(options) == 0 {
		return "", fmt.Errorf("no provider configures %s", agent)
	}

	providerName := defaultProviderName(agent, providerRegistry, current)
	err := huh.NewSelect[string]().
		Title("选择 Provider").
		Options(options...).
		Value(&providerName).
		Run()
	return providerName, err
}

func providerOptions(agent switcher.Agent, providerRegistry *registry.Registry, current switcher.CurrentState) []huh.Option[string] {
	options := make([]huh.Option[string], 0, len(providerRegistry.Providers))
	for _, name := range providerRegistry.Names() {
		provider := providerRegistry.Providers[name]
		label, ok := providerLabel(agent, name, provider, current)
		if !ok {
			continue
		}
		options = append(options, huh.NewOption(label, name))
	}
	return options
}

func providerLabel(agent switcher.Agent, name string, provider registry.Provider, current switcher.CurrentState) (string, bool) {
	marker := currentMarker(agent, name, current)
	switch agent {
	case switcher.AgentCodex:
		config, ok := provider.EffectiveCodex()
		if !ok {
			return "", false
		}
		return fmt.Sprintf("%s%s  %s", name, marker, config.BaseURL), true
	case switcher.AgentClaude:
		config, ok := provider.EffectiveClaude()
		if !ok {
			return "", false
		}
		return fmt.Sprintf("%s%s  %s", name, marker, config.BaseURL), true
	case switcher.AgentAll:
		codexConfig, supportsCodex := provider.EffectiveCodex()
		claudeConfig, supportsClaude := provider.EffectiveClaude()
		if !supportsCodex || !supportsClaude {
			return "", false
		}
		if provider.ConfigMode() == registry.ConfigModeUniversal {
			return fmt.Sprintf("%s%s  Universal: %s", name, marker, codexConfig.BaseURL), true
		}
		return fmt.Sprintf("%s%s  Codex: %s  Claude: %s", name, marker, codexConfig.BaseURL, claudeConfig.BaseURL), true
	default:
		return "", false
	}
}

func defaultProviderName(agent switcher.Agent, providerRegistry *registry.Registry, current switcher.CurrentState) string {
	candidates := current.Codex
	if agent == switcher.AgentClaude {
		candidates = current.Claude
	}
	for _, name := range candidates {
		provider, ok := providerRegistry.Providers[name]
		if !ok {
			continue
		}
		switch agent {
		case switcher.AgentCodex:
			if _, ok := provider.EffectiveCodex(); ok {
				return name
			}
		case switcher.AgentClaude:
			if _, ok := provider.EffectiveClaude(); ok {
				return name
			}
		case switcher.AgentAll:
			_, supportsCodex := provider.EffectiveCodex()
			_, supportsClaude := provider.EffectiveClaude()
			if supportsCodex && supportsClaude && containsName(current.Claude, name) {
				return name
			}
		}
	}
	return ""
}

func currentMarker(agent switcher.Agent, name string, current switcher.CurrentState) string {
	codexCurrent := containsName(current.Codex, name)
	claudeCurrent := containsName(current.Claude, name)

	switch agent {
	case switcher.AgentCodex:
		if codexCurrent {
			return " [current]"
		}
	case switcher.AgentClaude:
		if claudeCurrent {
			return " [current]"
		}
	case switcher.AgentAll:
		switch {
		case codexCurrent && claudeCurrent:
			return " [current]"
		case codexCurrent:
			return " [Codex current]"
		case claudeCurrent:
			return " [Claude current]"
		}
	}
	return ""
}

func containsName(names []string, target string) bool {
	for _, name := range names {
		if name == target {
			return true
		}
	}
	return false
}

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

func (Selector) SelectProvider(agent switcher.Agent, providerRegistry *registry.Registry) (string, error) {
	options := providerOptions(agent, providerRegistry)
	if len(options) == 0 {
		return "", fmt.Errorf("no provider configures %s", agent)
	}

	var providerName string
	err := huh.NewSelect[string]().
		Title("选择 Provider").
		Options(options...).
		Value(&providerName).
		Run()
	return providerName, err
}

func providerOptions(agent switcher.Agent, providerRegistry *registry.Registry) []huh.Option[string] {
	options := make([]huh.Option[string], 0, len(providerRegistry.Providers))
	for _, name := range providerRegistry.Names() {
		provider, err := providerRegistry.Provider(name)
		if err != nil {
			continue
		}
		label, ok := providerLabel(agent, name, provider)
		if !ok {
			continue
		}
		options = append(options, huh.NewOption(label, name))
	}
	return options
}

func providerLabel(agent switcher.Agent, name string, provider registry.Provider) (string, bool) {
	switch agent {
	case switcher.AgentCodex:
		if provider.Codex == nil {
			return "", false
		}
		return fmt.Sprintf("%s  %s  %s", name, displayModel(provider.Codex.Model), provider.Codex.BaseURL), true
	case switcher.AgentClaude:
		if provider.Claude == nil {
			return "", false
		}
		return fmt.Sprintf("%s  %s  %s", name, displayModel(provider.Claude.Model), provider.Claude.BaseURL), true
	case switcher.AgentAll:
		if provider.Codex == nil || provider.Claude == nil {
			return "", false
		}
		return fmt.Sprintf("%s  Codex: %s %s  Claude: %s %s", name, displayModel(provider.Codex.Model), provider.Codex.BaseURL, displayModel(provider.Claude.Model), provider.Claude.BaseURL), true
	default:
		return "", false
	}
}

func displayModel(model string) string {
	if model == "" {
		return "-"
	}
	return model
}

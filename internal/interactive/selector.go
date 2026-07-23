package interactive

import (
	"fmt"

	"charm.land/huh/v2"
	"github.com/shellus/ags/internal/agent"
	"github.com/shellus/ags/internal/app"
	"github.com/shellus/ags/internal/registry"
	"github.com/shellus/ags/internal/switcher"
)

type Selector struct{}

func (Selector) SelectMainAction() (string, error) {
	var action string
	err := huh.NewSelect[string]().
		Title("选择操作").
		Options(
			huh.NewOption("应用 Agent 环境", app.ActionEnvironmentApply),
			huh.NewOption("配置环境来源、Profile 和 Agent", app.ActionEnvironmentConfigure),
			huh.NewOption("查看环境状态", app.ActionEnvironmentStatus),
			huh.NewOption("安装 Agent", app.ActionAgentInstall),
			huh.NewOption("卸载 Agent", app.ActionAgentUninstall),
			huh.NewOption("切换 Provider", app.ActionProviderSwitch),
			huh.NewOption("环境检查", app.ActionDoctor),
			huh.NewOption("更新 AGS", app.ActionSelfUpdate),
		).
		Value(&action).
		Run()
	return action, err
}

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

func (Selector) SelectAgents(title string, selected []agent.Name) ([]agent.Name, error) {
	selectedSet := map[agent.Name]bool{}
	for _, name := range selected {
		selectedSet[name] = true
	}
	options := []huh.Option[agent.Name]{
		huh.NewOption("Codex", agent.Codex).Selected(selectedSet[agent.Codex]),
		huh.NewOption("Claude Code", agent.Claude).Selected(selectedSet[agent.Claude]),
	}
	values := append([]agent.Name{}, selected...)
	err := huh.NewMultiSelect[agent.Name]().
		Title(title).
		Options(options...).
		Value(&values).
		Run()
	return values, err
}

func (Selector) ConfigureEnvironment(profiles []string, currentProfile string, currentAgents []agent.Name) (string, []agent.Name, error) {
	profileOptions := make([]huh.Option[string], 0, len(profiles))
	for _, name := range profiles {
		profileOptions = append(profileOptions, huh.NewOption(name, name))
	}
	if currentProfile == "" && len(profiles) > 0 {
		currentProfile = profiles[0]
	}
	selectedSet := map[agent.Name]bool{}
	for _, name := range currentAgents {
		selectedSet[name] = true
	}
	selectedAgents := append([]agent.Name{}, currentAgents...)
	form := huh.NewForm(huh.NewGroup(
		huh.NewSelect[string]().
			Title("选择 Profile").
			Options(profileOptions...).
			Value(&currentProfile),
		huh.NewMultiSelect[agent.Name]().
			Title("选择本机管理的 Agent").
			Options(
				huh.NewOption("Codex", agent.Codex).Selected(selectedSet[agent.Codex]),
				huh.NewOption("Claude Code", agent.Claude).Selected(selectedSet[agent.Claude]),
			).
			Value(&selectedAgents),
	))
	if err := form.Run(); err != nil {
		return "", nil, err
	}
	if len(selectedAgents) == 0 {
		return "", nil, fmt.Errorf("at least one agent must be selected")
	}
	return currentProfile, selectedAgents, nil
}

func (Selector) InputSource(currentSource, currentBranch string) (string, string, error) {
	if currentBranch == "" {
		currentBranch = "main"
	}
	form := huh.NewForm(huh.NewGroup(
		huh.NewInput().
			Title("Agent Environment Git URL 或本地路径").
			Value(&currentSource),
		huh.NewInput().
			Title("分支").
			Value(&currentBranch),
	))
	if err := form.Run(); err != nil {
		return "", "", err
	}
	if currentSource == "" {
		return "", "", fmt.Errorf("environment source must not be empty")
	}
	return currentSource, currentBranch, nil
}

func (Selector) Confirm(title string) (bool, error) {
	confirmed := false
	err := huh.NewConfirm().
		Title(title).
		Affirmative("确认").
		Negative("取消").
		Value(&confirmed).
		Run()
	return confirmed, err
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
		provider, err := providerRegistry.Provider(name)
		if err != nil {
			continue
		}
		mode := providerRegistry.Providers[name].ConfigMode()
		label, ok := providerLabel(agent, name, provider, mode, current)
		if !ok {
			continue
		}
		options = append(options, huh.NewOption(label, name))
	}
	return options
}

func providerLabel(agent switcher.Agent, name string, provider registry.Provider, mode registry.ConfigMode, current switcher.CurrentState) (string, bool) {
	marker := currentMarker(agent, name, current)
	switch agent {
	case switcher.AgentCodex:
		if provider.Codex == nil {
			return "", false
		}
		return fmt.Sprintf("%s%s  %s  %s", name, marker, displayModel(provider.Codex.Model), provider.Codex.BaseURL), true
	case switcher.AgentClaude:
		if provider.Claude == nil {
			return "", false
		}
		return fmt.Sprintf("%s%s  %s  %s", name, marker, displayModel(provider.Claude.Model), provider.Claude.BaseURL), true
	case switcher.AgentAll:
		if provider.Codex == nil || provider.Claude == nil {
			return "", false
		}
		if mode == registry.ConfigModeUniversal {
			return fmt.Sprintf("%s%s  Universal: %s  Codex model: %s  Claude model: %s", name, marker, provider.Codex.BaseURL, displayModel(provider.Codex.Model), displayModel(provider.Claude.Model)), true
		}
		return fmt.Sprintf("%s%s  Codex: %s %s  Claude: %s %s", name, marker, displayModel(provider.Codex.Model), provider.Codex.BaseURL, displayModel(provider.Claude.Model), provider.Claude.BaseURL), true
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

func defaultProviderName(agent switcher.Agent, providerRegistry *registry.Registry, current switcher.CurrentState) string {
	candidates := current.Codex
	if agent == switcher.AgentClaude {
		candidates = current.Claude
	}
	for _, name := range candidates {
		provider, err := providerRegistry.Provider(name)
		if err != nil {
			continue
		}
		switch agent {
		case switcher.AgentCodex:
			if provider.Codex != nil {
				return name
			}
		case switcher.AgentClaude:
			if provider.Claude != nil {
				return name
			}
		case switcher.AgentAll:
			if provider.Codex != nil && provider.Claude != nil && containsName(current.Claude, name) {
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

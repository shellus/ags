package switcher

import (
	"fmt"

	"github.com/shellus/ags/internal/configfile"
	"github.com/shellus/ags/internal/registry"
	"github.com/shellus/ags/internal/transaction"
)

type Agent string

const (
	AgentCodex  Agent = "codex"
	AgentClaude Agent = "claude"
	AgentAll    Agent = "all"
)

type Service struct {
	Paths    configfile.Paths
	Registry *registry.Registry
}

type CurrentState struct {
	Codex  []string
	Claude []string
}

func ParseAgent(value string) (Agent, error) {
	switch Agent(value) {
	case AgentCodex, AgentClaude, AgentAll:
		return Agent(value), nil
	default:
		return "", fmt.Errorf("unknown agent %q; expected codex, claude, or all", value)
	}
}

func (s Service) Switch(agent Agent, providerName string) error {
	provider, err := s.Registry.Provider(providerName)
	if err != nil {
		return err
	}

	changes, err := s.prepare(agent, providerName, provider)
	if err != nil {
		return err
	}
	if err := transaction.Apply(changes); err != nil {
		return fmt.Errorf("apply provider %q for %s: %w", providerName, agent, err)
	}
	return nil
}

func (s Service) Current() (CurrentState, error) {
	codexAPIKey, codexBaseURL, err := readCodexCurrent(s.Paths)
	if err != nil {
		return CurrentState{}, err
	}
	claudeAuthToken, claudeBaseURL, err := readClaudeCurrent(s.Paths)
	if err != nil {
		return CurrentState{}, err
	}

	state := CurrentState{}
	for _, name := range s.Registry.Names() {
		provider := s.Registry.Providers[name]
		codexConfig, supportsCodex := provider.EffectiveCodex()
		if supportsCodex && codexConfig.APIKey == codexAPIKey && codexConfig.BaseURL == codexBaseURL {
			state.Codex = append(state.Codex, name)
		}
		claudeConfig, supportsClaude := provider.EffectiveClaude()
		if supportsClaude && claudeConfig.AuthToken == claudeAuthToken && claudeConfig.BaseURL == claudeBaseURL {
			state.Claude = append(state.Claude, name)
		}
	}
	return state, nil
}

func (s Service) prepare(agent Agent, providerName string, provider registry.Provider) ([]transaction.Change, error) {
	var changes []transaction.Change

	if agent == AgentCodex || agent == AgentAll {
		codexConfig, ok := provider.EffectiveCodex()
		if !ok {
			return nil, fmt.Errorf("provider %q does not configure codex", providerName)
		}
		codexChanges, err := prepareCodex(s.Paths, codexConfig)
		if err != nil {
			return nil, err
		}
		changes = append(changes, codexChanges...)
	}

	if agent == AgentClaude || agent == AgentAll {
		claudeConfig, ok := provider.EffectiveClaude()
		if !ok {
			return nil, fmt.Errorf("provider %q does not configure claude", providerName)
		}
		claudeChanges, err := prepareClaude(s.Paths, claudeConfig)
		if err != nil {
			return nil, err
		}
		changes = append(changes, claudeChanges...)
	}

	return changes, nil
}

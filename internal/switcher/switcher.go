package switcher

import (
	"fmt"

	"github.com/shellus/ags/internal/agent"
	"github.com/shellus/ags/internal/configfile"
	"github.com/shellus/ags/internal/registry"
	"github.com/shellus/ags/internal/transaction"
)

type Agent = agent.Name

const (
	AgentCodex  = agent.Codex
	AgentClaude = agent.Claude
	AgentAll    = agent.All
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
	return agent.Parse(value, true)
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
	codexAPIKey, codexBaseURL, codexModel, err := readCodexCurrent(s.Paths)
	if err != nil {
		return CurrentState{}, err
	}
	claudeAuthToken, claudeBaseURL, claudeModel, err := readClaudeCurrent(s.Paths)
	if err != nil {
		return CurrentState{}, err
	}

	state := CurrentState{}
	for _, name := range s.Registry.Names() {
		provider, err := s.Registry.Provider(name)
		if err != nil {
			return CurrentState{}, err
		}
		if provider.Codex != nil && provider.Codex.APIKey == codexAPIKey && provider.Codex.BaseURL == codexBaseURL && (provider.Codex.Model == "" || provider.Codex.Model == codexModel) {
			state.Codex = append(state.Codex, name)
		}
		if provider.Claude != nil && provider.Claude.AuthToken == claudeAuthToken && provider.Claude.BaseURL == claudeBaseURL && (provider.Claude.Model == "" || provider.Claude.Model == claudeModel) {
			state.Claude = append(state.Claude, name)
		}
	}
	return state, nil
}

func (s Service) prepare(agent Agent, providerName string, provider registry.Provider) ([]transaction.Change, error) {
	var changes []transaction.Change

	if agent == AgentCodex || agent == AgentAll {
		if provider.Codex == nil {
			return nil, fmt.Errorf("provider %q does not configure codex", providerName)
		}
		codexChanges, err := prepareCodex(s.Paths, *provider.Codex)
		if err != nil {
			return nil, err
		}
		changes = append(changes, codexChanges...)
	}

	if agent == AgentClaude || agent == AgentAll {
		if provider.Claude == nil {
			return nil, fmt.Errorf("provider %q does not configure claude", providerName)
		}
		claudeChanges, err := prepareClaude(s.Paths, *provider.Claude)
		if err != nil {
			return nil, err
		}
		changes = append(changes, claudeChanges...)
	}

	return changes, nil
}

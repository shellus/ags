package environment

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/shellus/ags/internal/agent"
)

const stateVersion = 1

type State struct {
	Version int                       `json:"version"`
	Source  string                    `json:"source"`
	Branch  string                    `json:"branch"`
	Commit  string                    `json:"commit"`
	Profile string                    `json:"profile"`
	Agents  map[agent.Name]AgentState `json:"agents"`
}

type AgentState struct {
	Version         string            `json:"version"`
	InstructionHash string            `json:"instruction_hash"`
	Skills          map[string]string `json:"skills"`
}

type managedMarker struct {
	Version int               `json:"version"`
	Skills  map[string]string `json:"skills"`
}

func LoadState(path string) (State, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return State{Version: stateVersion, Agents: map[agent.Name]AgentState{}}, nil
	}
	if err != nil {
		return State{}, fmt.Errorf("read environment state: %w", err)
	}
	var state State
	if err := json.Unmarshal(data, &state); err != nil {
		return State{}, fmt.Errorf("parse environment state: %w", err)
	}
	if state.Version != stateVersion {
		return State{}, fmt.Errorf("unsupported environment state version %d", state.Version)
	}
	if state.Agents == nil {
		state.Agents = map[agent.Name]AgentState{}
	}
	return state, nil
}

func saveState(path string, state State) error {
	state.Version = stateVersion
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return writeRegularFile(path, data, 0o600)
}

func readMarker(skillsDir string) (managedMarker, error) {
	path := filepath.Join(skillsDir, ".ags-managed.json")
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return managedMarker{Version: stateVersion, Skills: map[string]string{}}, nil
	}
	if err != nil {
		return managedMarker{}, err
	}
	var marker managedMarker
	if err := json.Unmarshal(data, &marker); err != nil {
		return managedMarker{}, fmt.Errorf("parse %s: %w", path, err)
	}
	if marker.Version != stateVersion {
		return managedMarker{}, fmt.Errorf("unsupported managed marker version %d", marker.Version)
	}
	if marker.Skills == nil {
		marker.Skills = map[string]string{}
	}
	return marker, nil
}

func writeMarker(skillsDir string, skills map[string]string) error {
	marker := managedMarker{Version: stateVersion, Skills: skills}
	data, err := json.MarshalIndent(marker, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return writeRegularFile(filepath.Join(skillsDir, ".ags-managed.json"), data, 0o600)
}

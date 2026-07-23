package environment

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/shellus/ags/internal/agent"
	"github.com/shellus/ags/internal/command"
	"gopkg.in/yaml.v3"
)

func UpdateLock(root string, runner command.Runner) (Lock, error) {
	if runner == nil {
		runner = command.OSRunner{}
	}
	repo, err := LoadRepository(root)
	if err != nil {
		return Lock{}, err
	}
	updated := Lock{Version: CurrentVersion, Agents: map[agent.Name]AgentLock{}, Sources: map[string]SourceLock{}}
	manager := agent.Manager{Runner: runner}
	for _, name := range agent.AllNames() {
		spec := repo.Manifest.Agents[name]
		version, err := manager.LatestVersion(spec.Package)
		if err != nil {
			return Lock{}, fmt.Errorf("resolve latest %s version: %w", name, err)
		}
		updated.Agents[name] = AgentLock{Version: version}
	}
	sourceNames := make([]string, 0, len(repo.Manifest.Sources))
	for name := range repo.Manifest.Sources {
		sourceNames = append(sourceNames, name)
	}
	sort.Strings(sourceNames)
	for _, name := range sourceNames {
		source := repo.Manifest.Sources[name]
		if source.Type != "git" {
			continue
		}
		lockName := sourceLockName(name, source)
		if _, exists := updated.Sources[lockName]; exists {
			continue
		}
		output, err := runner.Run("", nil, "git", "ls-remote", source.URL, "HEAD")
		if err != nil {
			return Lock{}, fmt.Errorf("resolve latest source %s: %w", name, err)
		}
		fields := strings.Fields(string(output))
		if len(fields) < 1 || len(fields[0]) != 40 {
			return Lock{}, fmt.Errorf("git ls-remote returned an invalid commit for %s", name)
		}
		updated.Sources[lockName] = SourceLock{Commit: fields[0]}
	}
	data, err := yaml.Marshal(updated)
	if err != nil {
		return Lock{}, err
	}
	if err := writeRegularFile(filepath.Join(root, "environment.lock"), data, 0o644); err != nil {
		return Lock{}, err
	}
	return updated, nil
}

func ValidateRepository(root string) error {
	repo, err := LoadRepository(root)
	if err != nil {
		return err
	}
	if _, err := os.Stat(filepath.Join(repo.Root, filepath.FromSlash(repo.Manifest.Instructions.Global))); err != nil {
		return fmt.Errorf("validate global instructions: %w", err)
	}
	return ValidatePublishedSkills(repo)
}

package environment

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/shellus/ags/internal/agent"
	"github.com/shellus/ags/internal/command"
	"github.com/shellus/ags/internal/configfile"
	"github.com/shellus/ags/internal/localconfig"
)

type Service struct {
	Paths        configfile.Paths
	Runner       command.Runner
	AgentManager agent.Manager
}

func (s Service) runner() command.Runner {
	if s.Runner == nil {
		return command.OSRunner{}
	}
	return s.Runner
}

func (s Service) manager() agent.Manager {
	if s.AgentManager.Runner == nil {
		s.AgentManager.Runner = s.runner()
	}
	return s.AgentManager
}

func (s Service) Prepare(config localconfig.Config, overrideAgents []agent.Name, overrideProfile string) (Plan, error) {
	agents := config.Environment.Agents
	if len(overrideAgents) > 0 {
		agents = overrideAgents
	}
	agents, err := agent.Expand(agents)
	if err != nil {
		return Plan{}, err
	}
	if len(agents) == 0 {
		return Plan{}, fmt.Errorf("no agents are configured for this machine")
	}
	profile := config.Environment.Profile
	if overrideProfile != "" {
		profile = overrideProfile
	}
	if profile == "" {
		return Plan{}, fmt.Errorf("environment profile is not configured")
	}
	repositoryManager := RepositoryManager{CacheDir: s.Paths.CacheDir, Runner: s.runner()}
	root, commit, err := repositoryManager.Sync(config.Environment.Source, config.Environment.Branch)
	if err != nil {
		return Plan{}, err
	}
	repo, err := LoadRepository(root)
	if err != nil {
		return Plan{}, err
	}
	compiler := Compiler{CacheDir: s.Paths.CacheDir, Runner: s.runner()}
	build, err := compiler.Build(repo, profile, agents)
	if err != nil {
		return Plan{}, err
	}
	installed := map[agent.Name]string{}
	manager := s.manager()
	for _, name := range agents {
		pkg, err := repo.AgentPackage(name)
		if err != nil {
			build.Cleanup()
			return Plan{}, err
		}
		version, err := manager.InstalledVersion(pkg)
		if err != nil {
			build.Cleanup()
			return Plan{}, err
		}
		installed[name] = version
	}
	plan, err := preparePlan(s.Paths, config, config.Environment.Source, config.Environment.Branch, commit, profile, repo, build, installed)
	if err != nil {
		build.Cleanup()
		return Plan{}, err
	}
	return plan, nil
}

func (s Service) Apply(plan Plan) error {
	if plan.HasConflicts() {
		return fmt.Errorf("environment has unmanaged Skill conflicts")
	}
	manager := s.manager()
	type packageRollback struct {
		pkg     agent.Package
		version string
	}
	var packageRollbacks []packageRollback
	rollbackPackages := func() error {
		var rollbackErr error
		for index := len(packageRollbacks) - 1; index >= 0; index-- {
			item := packageRollbacks[index]
			if item.version == "" {
				if err := manager.Uninstall(item.pkg); err != nil {
					rollbackErr = errors.Join(rollbackErr, err)
				}
				continue
			}
			item.pkg.Version = item.version
			if err := manager.Install(item.pkg); err != nil {
				rollbackErr = errors.Join(rollbackErr, err)
			}
		}
		return rollbackErr
	}

	for _, item := range plan.Agents {
		if item.InstalledVersion == item.DesiredVersion {
			continue
		}
		packageRollbacks = append(packageRollbacks, packageRollback{pkg: item.Package, version: item.InstalledVersion})
		if err := manager.Install(item.Package); err != nil {
			_ = rollbackPackages()
			return err
		}
	}

	backups, err := s.backupManaged(plan)
	if err != nil {
		_ = rollbackPackages()
		return err
	}
	if err := s.applyManaged(plan); err != nil {
		fileRollbackErr := s.restoreManaged(backups)
		packageRollbackErr := rollbackPackages()
		return errors.Join(err, fileRollbackErr, packageRollbackErr)
	}

	statePath := filepath.Join(s.Paths.StateDir, "environment.json")
	state, err := LoadState(statePath)
	if err != nil {
		return err
	}
	state.Version = stateVersion
	state.Source = plan.Source
	state.Branch = plan.Branch
	state.Commit = plan.Commit
	state.Profile = plan.Profile
	for _, item := range plan.Agents {
		state.Agents[item.Name] = AgentState{
			Version:         item.DesiredVersion,
			InstructionHash: bytesHash(plan.Build.Instruction),
			Skills:          item.DesiredSkills,
		}
	}
	if err := saveState(statePath, state); err != nil {
		return fmt.Errorf("save environment state: %w", err)
	}
	return nil
}

func (s Service) Uninstall(name agent.Name, pkg agent.Package, purge bool) error {
	statePath := filepath.Join(s.Paths.StateDir, "environment.json")
	state, err := LoadState(statePath)
	if err != nil {
		return err
	}
	_, managedAgent := state.Agents[name]
	_, skillsDir, err := agentDestinations(s.Paths, name)
	if err != nil {
		return err
	}
	marker, err := readMarker(skillsDir)
	if err != nil {
		return err
	}
	for skillName := range marker.Skills {
		if err := os.RemoveAll(filepath.Join(skillsDir, skillName)); err != nil {
			return err
		}
	}
	if err := os.Remove(filepath.Join(skillsDir, ".ags-managed.json")); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if managedAgent {
		guidance, _, _ := agentDestinations(s.Paths, name)
		if err := os.Remove(guidance); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	if err := s.manager().Uninstall(pkg); err != nil {
		return err
	}
	if purge {
		var configDir string
		switch name {
		case agent.Codex:
			configDir = s.Paths.CodexDir
		case agent.Claude:
			configDir = s.Paths.ClaudeDir
		}
		if configDir != "" {
			if err := os.RemoveAll(configDir); err != nil {
				return fmt.Errorf("purge %s config: %w", name, err)
			}
		}
	}
	delete(state.Agents, name)
	if err := saveState(statePath, state); err != nil {
		return err
	}
	return nil
}

type agentBackup struct {
	Name              agent.Name
	Root              string
	InstructionExists bool
	InstructionMode   os.FileMode
	Skills            []string
	MarkerExists      bool
}

func (s Service) backupManaged(plan Plan) ([]agentBackup, error) {
	backupRoot := filepath.Join(plan.Build.Root, "backup")
	var backups []agentBackup
	for _, item := range plan.Agents {
		root := filepath.Join(backupRoot, string(item.Name))
		backup := agentBackup{Name: item.Name, Root: root}
		if info, err := os.Stat(item.InstructionDestination); err == nil {
			backup.InstructionExists = true
			backup.InstructionMode = info.Mode().Perm()
			if err := copyFile(item.InstructionDestination, filepath.Join(root, "instruction"), info.Mode().Perm()); err != nil {
				return nil, err
			}
		} else if !errors.Is(err, os.ErrNotExist) {
			return nil, err
		}
		for skillName := range item.ManagedBefore {
			source := filepath.Join(item.SkillsDestination, skillName)
			if _, err := os.Stat(source); errors.Is(err, os.ErrNotExist) {
				continue
			} else if err != nil {
				return nil, err
			}
			if err := copyTree(source, filepath.Join(root, "skills", skillName)); err != nil {
				return nil, err
			}
			backup.Skills = append(backup.Skills, skillName)
		}
		marker := filepath.Join(item.SkillsDestination, ".ags-managed.json")
		if info, err := os.Stat(marker); err == nil {
			backup.MarkerExists = true
			if err := copyFile(marker, filepath.Join(root, "marker"), info.Mode().Perm()); err != nil {
				return nil, err
			}
		}
		backups = append(backups, backup)
	}
	return backups, nil
}

func (s Service) applyManaged(plan Plan) error {
	for _, item := range plan.Agents {
		if err := writeRegularFile(item.InstructionDestination, plan.Build.Instruction, 0o644); err != nil {
			return fmt.Errorf("write %s instructions: %w", item.Name, err)
		}
		if err := os.MkdirAll(item.SkillsDestination, 0o755); err != nil {
			return err
		}
		for skillName := range item.ManagedBefore {
			if err := os.RemoveAll(filepath.Join(item.SkillsDestination, skillName)); err != nil {
				return err
			}
		}
		names := make([]string, 0, len(item.DesiredSkills))
		for name := range item.DesiredSkills {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, skillName := range names {
			if err := copyTree(filepath.Join(item.Stage, skillName), filepath.Join(item.SkillsDestination, skillName)); err != nil {
				return err
			}
		}
		if err := writeMarker(item.SkillsDestination, item.DesiredSkills); err != nil {
			return err
		}
	}
	return nil
}

func (s Service) restoreManaged(backups []agentBackup) error {
	var rollbackErr error
	for _, backup := range backups {
		guidance, skillsDir, err := agentDestinations(s.Paths, backup.Name)
		if err != nil {
			rollbackErr = errors.Join(rollbackErr, err)
			continue
		}
		if backup.InstructionExists {
			if err := copyFile(filepath.Join(backup.Root, "instruction"), guidance, backup.InstructionMode); err != nil {
				rollbackErr = errors.Join(rollbackErr, err)
			}
		} else if err := os.Remove(guidance); err != nil && !errors.Is(err, os.ErrNotExist) {
			rollbackErr = errors.Join(rollbackErr, err)
		}
		marker, _ := readMarker(skillsDir)
		for skillName := range marker.Skills {
			if err := os.RemoveAll(filepath.Join(skillsDir, skillName)); err != nil {
				rollbackErr = errors.Join(rollbackErr, err)
			}
		}
		for _, skillName := range backup.Skills {
			if err := copyTree(filepath.Join(backup.Root, "skills", skillName), filepath.Join(skillsDir, skillName)); err != nil {
				rollbackErr = errors.Join(rollbackErr, err)
			}
		}
		markerPath := filepath.Join(skillsDir, ".ags-managed.json")
		if backup.MarkerExists {
			if err := copyFile(filepath.Join(backup.Root, "marker"), markerPath, 0o600); err != nil {
				rollbackErr = errors.Join(rollbackErr, err)
			}
		} else if err := os.Remove(markerPath); err != nil && !errors.Is(err, os.ErrNotExist) {
			rollbackErr = errors.Join(rollbackErr, err)
		}
	}
	return rollbackErr
}

func writeRegularFile(path string, content []byte, mode os.FileMode) error {
	if info, err := os.Lstat(path); err == nil && info.Mode()&os.ModeSymlink != 0 {
		if err := os.Remove(path); err != nil {
			return err
		}
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".ags-write-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if err := tmp.Chmod(mode.Perm()); err != nil {
		tmp.Close()
		return err
	}
	if _, err := tmp.Write(content); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return os.Rename(tmpPath, path)
}

func copyFile(source, target string, mode os.FileMode) error {
	data, err := os.ReadFile(source)
	if err != nil {
		return err
	}
	return writeRegularFile(target, data, mode)
}

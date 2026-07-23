package environment

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/shellus/ags/internal/agent"
	"gopkg.in/yaml.v3"
)

const CurrentVersion = 1

type Manifest struct {
	Version      int                  `yaml:"version"`
	Instructions InstructionConfig    `yaml:"instructions"`
	Agents       map[agent.Name]Agent `yaml:"agents"`
	Sources      map[string]Source    `yaml:"sources"`
	Profiles     map[string]Profile   `yaml:"profiles"`
	Root         string               `yaml:"-"`
}

type InstructionConfig struct {
	Global string `yaml:"global"`
}

type Agent struct {
	Package string `yaml:"package"`
}

type Source struct {
	Type     string   `yaml:"type"`
	Path     string   `yaml:"path,omitempty"`
	URL      string   `yaml:"url,omitempty"`
	Lock     string   `yaml:"lock,omitempty"`
	Discover Discover `yaml:"discover"`
	Build    *Build   `yaml:"build,omitempty"`
	Patches  []string `yaml:"patches,omitempty"`
}

type Discover struct {
	Mode string `yaml:"mode"`
	Path string `yaml:"path,omitempty"`
}

type Build struct {
	Template     string              `yaml:"template,omitempty"`
	Output       string              `yaml:"output,omitempty"`
	Replacements map[string]string   `yaml:"replacements,omitempty"`
	Commands     map[string][]string `yaml:"commands,omitempty"`
}

type Profile struct {
	Include []string                    `yaml:"include"`
	Exclude []string                    `yaml:"exclude,omitempty"`
	Agents  map[agent.Name]AgentProfile `yaml:"agents"`
}

type AgentProfile struct {
	Include  []string `yaml:"include,omitempty"`
	Exclude  []string `yaml:"exclude,omitempty"`
	Preserve []string `yaml:"preserve,omitempty"`
}

type Lock struct {
	Version int                      `yaml:"version"`
	Agents  map[agent.Name]AgentLock `yaml:"agents"`
	Sources map[string]SourceLock    `yaml:"sources"`
}

type AgentLock struct {
	Version string `yaml:"version"`
}

type SourceLock struct {
	Commit string `yaml:"commit"`
}

type Repository struct {
	Root     string
	Manifest Manifest
	Lock     Lock
}

func LoadRepository(root string) (Repository, error) {
	root, err := filepath.Abs(root)
	if err != nil {
		return Repository{}, fmt.Errorf("resolve environment repository: %w", err)
	}
	manifestPath := filepath.Join(root, "environment.yaml")
	lockPath := filepath.Join(root, "environment.lock")
	var manifest Manifest
	if err := loadYAML(manifestPath, &manifest); err != nil {
		return Repository{}, err
	}
	manifest.Root = root
	var lock Lock
	if err := loadYAML(lockPath, &lock); err != nil {
		return Repository{}, err
	}
	repo := Repository{Root: root, Manifest: manifest, Lock: lock}
	if err := repo.Validate(); err != nil {
		return Repository{}, err
	}
	return repo, nil
}

func loadYAML(path string, target any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}
	decoder := yaml.NewDecoder(strings.NewReader(string(data)))
	decoder.KnownFields(true)
	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("parse %s: %w", path, err)
	}
	return nil
}

func (r Repository) Validate() error {
	if r.Manifest.Version != CurrentVersion || r.Lock.Version != CurrentVersion {
		return fmt.Errorf("environment and lock versions must both be %d", CurrentVersion)
	}
	if strings.TrimSpace(r.Manifest.Instructions.Global) == "" {
		return fmt.Errorf("instructions.global must not be empty")
	}
	if !withinRoot(r.Root, filepath.Join(r.Root, filepath.FromSlash(r.Manifest.Instructions.Global))) {
		return fmt.Errorf("instructions.global leaves repository root")
	}
	for _, name := range agent.AllNames() {
		spec, ok := r.Manifest.Agents[name]
		if !ok || strings.TrimSpace(spec.Package) == "" {
			return fmt.Errorf("agent %s package must be configured", name)
		}
		locked, ok := r.Lock.Agents[name]
		if !ok || strings.TrimSpace(locked.Version) == "" {
			return fmt.Errorf("agent %s version must be locked", name)
		}
	}
	if len(r.Manifest.Profiles) == 0 {
		return fmt.Errorf("profiles must contain at least one entry")
	}
	for name, source := range r.Manifest.Sources {
		if strings.TrimSpace(name) == "" {
			return fmt.Errorf("source name must not be empty")
		}
		switch source.Type {
		case "local":
			if strings.TrimSpace(source.Path) == "" {
				return fmt.Errorf("local source %s requires path", name)
			}
		case "git":
			if strings.TrimSpace(source.URL) == "" {
				return fmt.Errorf("git source %s requires url", name)
			}
			lockName := sourceLockName(name, source)
			if strings.TrimSpace(r.Lock.Sources[lockName].Commit) == "" {
				return fmt.Errorf("git source %s requires lock %s", name, lockName)
			}
		default:
			return fmt.Errorf("source %s has unsupported type %q", name, source.Type)
		}
		if source.Discover.Mode != "flat" && source.Discover.Mode != "single" && source.Discover.Mode != "collection" {
			return fmt.Errorf("source %s has unsupported discover mode %q", name, source.Discover.Mode)
		}
	}
	for profileName, profile := range r.Manifest.Profiles {
		if strings.TrimSpace(profileName) == "" {
			return fmt.Errorf("profile name must not be empty")
		}
		if len(profile.Agents) == 0 {
			return fmt.Errorf("profile %s must configure at least one agent", profileName)
		}
		for name := range profile.Agents {
			if _, err := agent.Parse(string(name), false); err != nil {
				return fmt.Errorf("profile %s: %w", profileName, err)
			}
		}
	}
	return nil
}

func (r Repository) ProfileNames() []string {
	names := make([]string, 0, len(r.Manifest.Profiles))
	for name := range r.Manifest.Profiles {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func (r Repository) AgentPackage(name agent.Name) (agent.Package, error) {
	spec, ok := r.Manifest.Agents[name]
	if !ok {
		return agent.Package{}, fmt.Errorf("environment does not configure agent %s", name)
	}
	locked, ok := r.Lock.Agents[name]
	if !ok {
		return agent.Package{}, fmt.Errorf("environment does not lock agent %s", name)
	}
	return agent.Package{Name: name, NPMName: spec.Package, Version: locked.Version}, nil
}

func (r Repository) Selection(profileName string, name agent.Name) (AgentProfile, []string, []string, error) {
	profile, ok := r.Manifest.Profiles[profileName]
	if !ok {
		return AgentProfile{}, nil, nil, fmt.Errorf("unknown profile %q", profileName)
	}
	agentProfile, ok := profile.Agents[name]
	if !ok {
		return AgentProfile{}, nil, nil, fmt.Errorf("profile %q does not configure %s", profileName, name)
	}
	include := append(append([]string{}, profile.Include...), agentProfile.Include...)
	exclude := append(append([]string{}, profile.Exclude...), agentProfile.Exclude...)
	return agentProfile, include, exclude, nil
}

func sourceLockName(name string, source Source) string {
	if strings.TrimSpace(source.Lock) != "" {
		return source.Lock
	}
	return name
}

func withinRoot(root, path string) bool {
	rel, err := filepath.Rel(root, path)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

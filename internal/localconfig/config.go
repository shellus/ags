package localconfig

import (
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/shellus/ags/internal/agent"
	"github.com/shellus/ags/internal/transaction"
	"gopkg.in/yaml.v3"
)

const CurrentVersion = 1

type Config struct {
	Version     int               `yaml:"version"`
	Environment EnvironmentConfig `yaml:"environment"`
}

type EnvironmentConfig struct {
	Source  string       `yaml:"source"`
	Branch  string       `yaml:"branch"`
	Profile string       `yaml:"profile"`
	Agents  []agent.Name `yaml:"agents"`
}

func Default() Config {
	return Config{
		Version: CurrentVersion,
		Environment: EnvironmentConfig{
			Branch: "main",
		},
	}
}

func Load(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return Default(), nil
	}
	if err != nil {
		return Config{}, fmt.Errorf("read AGS config %s: %w", path, err)
	}
	config := Default()
	decoder := yaml.NewDecoder(strings.NewReader(string(data)))
	decoder.KnownFields(true)
	if err := decoder.Decode(&config); err != nil {
		return Config{}, fmt.Errorf("parse AGS config %s: %w", path, err)
	}
	if err := config.Validate(); err != nil {
		return Config{}, fmt.Errorf("validate AGS config %s: %w", path, err)
	}
	return config, nil
}

func (c Config) Validate() error {
	if c.Version != CurrentVersion {
		return fmt.Errorf("unsupported config version %d", c.Version)
	}
	if strings.TrimSpace(c.Environment.Branch) == "" {
		return fmt.Errorf("environment.branch must not be empty")
	}
	seen := map[agent.Name]bool{}
	for _, name := range c.Environment.Agents {
		parsed, err := agent.Parse(string(name), false)
		if err != nil {
			return err
		}
		if seen[parsed] {
			return fmt.Errorf("duplicate environment agent %q", parsed)
		}
		seen[parsed] = true
	}
	return nil
}

func Save(path string, config Config) error {
	if err := config.Validate(); err != nil {
		return err
	}
	config.Environment.Source = strings.TrimSpace(config.Environment.Source)
	config.Environment.Branch = strings.TrimSpace(config.Environment.Branch)
	config.Environment.Profile = strings.TrimSpace(config.Environment.Profile)
	sort.SliceStable(config.Environment.Agents, func(i, j int) bool {
		return config.Environment.Agents[i] < config.Environment.Agents[j]
	})
	data, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("encode AGS config: %w", err)
	}
	return transaction.Apply([]transaction.Change{{Path: path, Content: data}})
}

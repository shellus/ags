package app

import (
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	"github.com/shellus/ags/internal/configfile"
	"github.com/shellus/ags/internal/registry"
	"github.com/shellus/ags/internal/switcher"
)

type Runner struct {
	Paths    configfile.Paths
	Out      io.Writer
	Selector Selector
}

type Selector interface {
	SelectAgent() (switcher.Agent, error)
	SelectProvider(switcher.Agent, *registry.Registry) (string, error)
}

type UsageError struct {
	Message string
}

func (e *UsageError) Error() string {
	return e.Message
}

func (r Runner) Run(args []string) error {
	if r.Out == nil {
		r.Out = io.Discard
	}
	if len(args) == 0 {
		return r.runInteractive("")
	}

	switch args[0] {
	case "help", "-h", "--help":
		if len(args) != 1 {
			return &UsageError{Message: "help does not accept arguments"}
		}
		r.printUsage()
		return nil
	case "list":
		if len(args) != 1 {
			return &UsageError{Message: "usage: ags list"}
		}
		providerRegistry, err := registry.Load(r.Paths.Registry)
		if err != nil {
			return err
		}
		r.printProviders(providerRegistry)
		return nil
	case "current":
		if len(args) != 1 {
			return &UsageError{Message: "usage: ags current"}
		}
		providerRegistry, err := registry.Load(r.Paths.Registry)
		if err != nil {
			return err
		}
		service := switcher.Service{Paths: r.Paths, Registry: providerRegistry}
		state, err := service.Current()
		if err != nil {
			return err
		}
		fmt.Fprintf(r.Out, "codex: %s\n", currentNames(state.Codex))
		fmt.Fprintf(r.Out, "claude: %s\n", currentNames(state.Claude))
		return nil
	default:
		agent, err := switcher.ParseAgent(args[0])
		if err != nil {
			return &UsageError{Message: err.Error()}
		}
		if len(args) == 1 {
			return r.runInteractive(agent)
		}
		if len(args) != 2 {
			return &UsageError{Message: "usage: ags <codex|claude|all> [provider]"}
		}
		providerRegistry, err := registry.Load(r.Paths.Registry)
		if err != nil {
			return err
		}
		service := switcher.Service{Paths: r.Paths, Registry: providerRegistry}
		if err := service.Switch(agent, args[1]); err != nil {
			return err
		}
		fmt.Fprintf(r.Out, "Switched %s provider to %s\n", agent, args[1])
		return nil
	}
}

func (r Runner) runInteractive(agent switcher.Agent) error {
	if r.Selector == nil {
		return fmt.Errorf("interactive selector is not configured")
	}
	providerRegistry, err := registry.Load(r.Paths.Registry)
	if err != nil {
		return err
	}
	if agent == "" {
		agent, err = r.Selector.SelectAgent()
		if err != nil {
			return fmt.Errorf("select agent: %w", err)
		}
	}
	providerName, err := r.Selector.SelectProvider(agent, providerRegistry)
	if err != nil {
		return fmt.Errorf("select provider for %s: %w", agent, err)
	}
	service := switcher.Service{Paths: r.Paths, Registry: providerRegistry}
	if err := service.Switch(agent, providerName); err != nil {
		return err
	}
	fmt.Fprintf(r.Out, "Switched %s provider to %s\n", agent, providerName)
	return nil
}

func (r Runner) printUsage() {
	fmt.Fprint(r.Out, `ags changes the active API provider for Codex and Claude Code.

Usage:
  ags
  ags <codex|claude|all>
  ags <codex|claude|all> <provider>
  ags list
  ags current
  ags help

Files:
  ~/.agent-switch/providers.yaml
  ~/.codex/auth.json
  ~/.codex/config.toml
  ~/.claude/settings.json
`)
}

func (r Runner) printProviders(providerRegistry *registry.Registry) {
	w := tabwriter.NewWriter(r.Out, 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "PROVIDER\tCODEX MODEL\tCODEX BASE URL\tCLAUDE MODEL\tCLAUDE BASE URL")
	for _, name := range providerRegistry.Names() {
		provider, err := providerRegistry.Provider(name)
		if err != nil {
			continue
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", name, codexModel(provider), codexBaseURL(provider), claudeModel(provider), claudeBaseURL(provider))
	}
	_ = w.Flush()
}

func codexModel(provider registry.Provider) string {
	if provider.Codex != nil && provider.Codex.Model != "" {
		return provider.Codex.Model
	}
	return "-"
}

func codexBaseURL(provider registry.Provider) string {
	if provider.Codex != nil {
		return provider.Codex.BaseURL
	}
	return "-"
}

func claudeBaseURL(provider registry.Provider) string {
	if provider.Claude != nil {
		return provider.Claude.BaseURL
	}
	return "-"
}

func claudeModel(provider registry.Provider) string {
	if provider.Claude != nil && provider.Claude.Model != "" {
		return provider.Claude.Model
	}
	return "-"
}

func currentNames(names []string) string {
	if len(names) == 0 {
		return "unknown"
	}
	return strings.Join(names, ", ")
}

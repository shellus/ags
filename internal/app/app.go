package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/shellus/ags/internal/agent"
	"github.com/shellus/ags/internal/command"
	"github.com/shellus/ags/internal/configfile"
	"github.com/shellus/ags/internal/environment"
	"github.com/shellus/ags/internal/localconfig"
	"github.com/shellus/ags/internal/registry"
	"github.com/shellus/ags/internal/selfupdate"
	"github.com/shellus/ags/internal/switcher"
	"github.com/shellus/ags/internal/version"
)

const (
	ActionEnvironmentApply     = "environment-apply"
	ActionEnvironmentConfigure = "environment-configure"
	ActionEnvironmentStatus    = "environment-status"
	ActionAgentInstall         = "agent-install"
	ActionAgentUninstall       = "agent-uninstall"
	ActionProviderSwitch       = "provider-switch"
	ActionDoctor               = "doctor"
	ActionSelfUpdate           = "self-update"
)

type Runner struct {
	Paths       configfile.Paths
	Out         io.Writer
	UI          UI
	Commands    command.Runner
	Interactive bool
}

type UI interface {
	SelectMainAction() (string, error)
	SelectAgent() (switcher.Agent, error)
	SelectAgents(title string, selected []agent.Name) ([]agent.Name, error)
	SelectProvider(switcher.Agent, *registry.Registry, switcher.CurrentState) (string, error)
	ConfigureEnvironment(profiles []string, currentProfile string, currentAgents []agent.Name) (string, []agent.Name, error)
	InputSource(currentSource, currentBranch string) (string, string, error)
	Confirm(title string) (bool, error)
}

type UsageError struct{ Message string }

func (e *UsageError) Error() string { return e.Message }

func (r Runner) commands() command.Runner {
	if r.Commands == nil {
		return command.OSRunner{}
	}
	return r.Commands
}

func (r Runner) environmentService() environment.Service {
	return environment.Service{
		Paths:        r.Paths,
		Runner:       r.commands(),
		AgentManager: agent.Manager{Runner: r.commands()},
	}
}

func (r Runner) Run(args []string) error {
	if r.Out == nil {
		r.Out = io.Discard
	}
	if len(args) == 0 {
		if !r.Interactive || r.UI == nil {
			return &UsageError{Message: "interactive terminal required; run ags help for commands"}
		}
		return r.runInteractive()
	}
	switch args[0] {
	case "help", "-h", "--help":
		if len(args) != 1 {
			return &UsageError{Message: "usage: ags help"}
		}
		r.printUsage()
		return nil
	case "env":
		return r.runEnvironment(args[1:])
	case "agent":
		return r.runAgent(args[1:])
	case "provider":
		return r.runProvider(args[1:])
	case "self":
		return r.runSelf(args[1:])
	case "doctor":
		if len(args) != 1 {
			return &UsageError{Message: "usage: ags doctor"}
		}
		return r.runDoctor()
	default:
		return &UsageError{Message: fmt.Sprintf("unknown command %q; run ags help", args[0])}
	}
}

func (r Runner) runInteractive() error {
	action, err := r.UI.SelectMainAction()
	if err != nil {
		return err
	}
	switch action {
	case ActionEnvironmentApply:
		return r.runEnvironment([]string{"apply"})
	case ActionEnvironmentConfigure:
		return r.runEnvironment([]string{"configure"})
	case ActionEnvironmentStatus:
		return r.runEnvironment([]string{"status"})
	case ActionAgentInstall:
		return r.runAgent([]string{"install"})
	case ActionAgentUninstall:
		return r.runAgent([]string{"uninstall"})
	case ActionProviderSwitch:
		return r.runProvider([]string{"switch"})
	case ActionDoctor:
		return r.runDoctor()
	case ActionSelfUpdate:
		return r.runSelf([]string{"update"})
	default:
		return fmt.Errorf("unknown interactive action %q", action)
	}
}

func (r Runner) runEnvironment(args []string) error {
	if len(args) == 0 {
		if !r.Interactive || r.UI == nil {
			return &UsageError{Message: "usage: ags env <source|configure|apply|status|diff|lock|vendor|validate>"}
		}
		return r.runEnvironment([]string{"apply"})
	}
	switch args[0] {
	case "source":
		return r.runEnvironmentSource(args[1:])
	case "configure":
		return r.runEnvironmentConfigure(args[1:])
	case "apply", "status", "diff":
		return r.runEnvironmentPlan(args[0], args[1:])
	case "validate":
		repo, err := repoOption(args[1:])
		if err != nil {
			return err
		}
		if err := environment.ValidateRepository(repo); err != nil {
			return err
		}
		fmt.Fprintf(r.Out, "Environment repository is valid: %s\n", repo)
		return nil
	case "lock":
		repo, err := repoOption(args[1:])
		if err != nil {
			return err
		}
		lock, err := environment.UpdateLock(repo, r.commands())
		if err != nil {
			return err
		}
		fmt.Fprintf(r.Out, "Updated environment lock: %s\n", filepath.Join(repo, "environment.lock"))
		for _, name := range agent.AllNames() {
			fmt.Fprintf(r.Out, "  %s: %s\n", name, lock.Agents[name].Version)
		}
		return nil
	case "vendor":
		repoPath, err := repoOption(args[1:])
		if err != nil {
			return err
		}
		repo, err := environment.LoadRepository(repoPath)
		if err != nil {
			return err
		}
		snapshot, err := (environment.Compiler{CacheDir: r.Paths.CacheDir, Runner: r.commands()}).Vendor(repo)
		if err != nil {
			return err
		}
		if err := environment.ValidateRepository(repoPath); err != nil {
			return err
		}
		fmt.Fprintf(r.Out, "Published %d vendored Skills to %s\n", len(snapshot.Skills), filepath.Join(repo.Root, filepath.FromSlash(repo.Manifest.Skills.Vendor)))
		return nil
	default:
		return &UsageError{Message: fmt.Sprintf("unknown env command %q", args[0])}
	}
}

func (r Runner) runEnvironmentSource(args []string) error {
	config, err := localconfig.Load(r.Paths.AGSConfig)
	if err != nil {
		return err
	}
	if len(args) == 1 && args[0] == "show" {
		fmt.Fprintf(r.Out, "source: %s\nbranch: %s\n", displayValue(config.Environment.Source), config.Environment.Branch)
		return nil
	}
	if len(args) == 0 && r.Interactive && r.UI != nil {
		source, branch, err := r.UI.InputSource(config.Environment.Source, config.Environment.Branch)
		if err != nil {
			return err
		}
		args = []string{"set", source, "--branch", branch}
	}
	if len(args) < 2 || args[0] != "set" {
		return &UsageError{Message: "usage: ags env source set <git-url-or-path> [--branch main]"}
	}
	source := args[1]
	branch := config.Environment.Branch
	for index := 2; index < len(args); index++ {
		if args[index] != "--branch" || index+1 >= len(args) {
			return &UsageError{Message: "usage: ags env source set <git-url-or-path> [--branch main]"}
		}
		branch = args[index+1]
		index++
	}
	manager := environment.RepositoryManager{CacheDir: r.Paths.CacheDir, Runner: r.commands()}
	root, commit, err := manager.Sync(source, branch)
	if err != nil {
		return err
	}
	repo, err := environment.LoadRepository(root)
	if err != nil {
		return err
	}
	config.Environment.Source = source
	config.Environment.Branch = branch
	if config.Environment.Profile == "" {
		for _, name := range repo.ProfileNames() {
			if name == "default" {
				config.Environment.Profile = name
				break
			}
		}
	}
	if err := localconfig.Save(r.Paths.AGSConfig, config); err != nil {
		return err
	}
	fmt.Fprintf(r.Out, "Environment source configured at %s (%s)\n", root, shortCommit(commit))
	return nil
}

func (r Runner) runEnvironmentConfigure(args []string) error {
	config, err := localconfig.Load(r.Paths.AGSConfig)
	if err != nil {
		return err
	}
	if config.Environment.Source == "" {
		if r.Interactive && r.UI != nil {
			if err := r.runEnvironmentSource(nil); err != nil {
				return err
			}
			config, err = localconfig.Load(r.Paths.AGSConfig)
			if err != nil {
				return err
			}
		} else {
			return fmt.Errorf("environment source is not configured")
		}
	}
	profile, agents, _, err := parseEnvironmentOptions(args)
	if err != nil {
		return err
	}
	manager := environment.RepositoryManager{CacheDir: r.Paths.CacheDir, Runner: r.commands()}
	root, _, err := manager.Sync(config.Environment.Source, config.Environment.Branch)
	if err != nil {
		return err
	}
	repo, err := environment.LoadRepository(root)
	if err != nil {
		return err
	}
	if profile == "" && len(agents) == 0 {
		if !r.Interactive || r.UI == nil {
			return &UsageError{Message: "usage: ags env configure --profile <name> --agents <codex,claude>"}
		}
		profile, agents, err = r.UI.ConfigureEnvironment(repo.ProfileNames(), config.Environment.Profile, config.Environment.Agents)
		if err != nil {
			return err
		}
	}
	if profile != "" {
		if _, ok := repo.Manifest.Profiles[profile]; !ok {
			return fmt.Errorf("unknown profile %q", profile)
		}
		config.Environment.Profile = profile
	}
	if len(agents) > 0 {
		config.Environment.Agents = agents
	}
	if config.Environment.Profile == "" || len(config.Environment.Agents) == 0 {
		return fmt.Errorf("profile and at least one agent must be configured")
	}
	if err := localconfig.Save(r.Paths.AGSConfig, config); err != nil {
		return err
	}
	fmt.Fprintf(r.Out, "Environment configured: profile=%s agents=%s\n", config.Environment.Profile, joinAgents(config.Environment.Agents))
	return nil
}

func (r Runner) runEnvironmentPlan(commandName string, args []string) error {
	config, err := localconfig.Load(r.Paths.AGSConfig)
	if err != nil {
		return err
	}
	if config.Environment.Source == "" || config.Environment.Profile == "" || len(config.Environment.Agents) == 0 {
		if r.Interactive && r.UI != nil {
			if err := r.runEnvironmentConfigure(nil); err != nil {
				return err
			}
			config, err = localconfig.Load(r.Paths.AGSConfig)
			if err != nil {
				return err
			}
		} else {
			return fmt.Errorf("environment source, profile, and agents must be configured")
		}
	}
	profile, agents, yes, err := parseEnvironmentOptions(args)
	if err != nil {
		return err
	}
	plan, err := r.environmentService().Prepare(config, agents, profile)
	if err != nil {
		return err
	}
	defer plan.Cleanup()
	r.printPlan(plan, commandName == "diff")
	if commandName != "apply" {
		return nil
	}
	if !plan.HasChanges() {
		fmt.Fprintln(r.Out, "Environment is already up to date.")
		return nil
	}
	if !yes {
		if !r.Interactive || r.UI == nil {
			return &UsageError{Message: "environment changes require --yes in non-interactive mode"}
		}
		confirmed, err := r.UI.Confirm("应用以上 Agent 环境变更？")
		if err != nil {
			return err
		}
		if !confirmed {
			return nil
		}
	}
	if err := r.environmentService().Apply(plan); err != nil {
		return err
	}
	fmt.Fprintf(r.Out, "Applied environment %s at %s\n", plan.Profile, shortCommit(plan.Commit))
	return nil
}

func (r Runner) runAgent(args []string) error {
	if len(args) == 0 {
		return &UsageError{Message: "usage: ags agent <install|uninstall|status>"}
	}
	switch args[0] {
	case "install":
		return r.runAgentInstall(args[1:])
	case "uninstall":
		return r.runAgentUninstall(args[1:])
	case "status":
		return r.runAgentStatus(args[1:])
	default:
		return &UsageError{Message: fmt.Sprintf("unknown agent command %q", args[0])}
	}
}

func (r Runner) desiredRepository() (localconfig.Config, environment.Repository, error) {
	config, err := localconfig.Load(r.Paths.AGSConfig)
	if err != nil {
		return localconfig.Config{}, environment.Repository{}, err
	}
	if config.Environment.Source == "" {
		return config, environment.Repository{}, fmt.Errorf("environment source is not configured")
	}
	manager := environment.RepositoryManager{CacheDir: r.Paths.CacheDir, Runner: r.commands()}
	root, _, err := manager.Sync(config.Environment.Source, config.Environment.Branch)
	if err != nil {
		return config, environment.Repository{}, err
	}
	repo, err := environment.LoadRepository(root)
	return config, repo, err
}

func (r Runner) runAgentInstall(args []string) error {
	_, repo, err := r.desiredRepository()
	if err != nil {
		return err
	}
	agents, _, err := parseAgentTargets(args, false)
	if err != nil {
		return err
	}
	if len(agents) == 0 {
		if !r.Interactive || r.UI == nil {
			return &UsageError{Message: "usage: ags agent install <codex|claude|all>"}
		}
		agents, err = r.UI.SelectAgents("选择要安装的 Agent", nil)
		if err != nil {
			return err
		}
	}
	agents, err = agent.Expand(agents)
	if err != nil {
		return err
	}
	manager := agent.Manager{Runner: r.commands()}
	for _, name := range agents {
		pkg, err := repo.AgentPackage(name)
		if err != nil {
			return err
		}
		installed, err := manager.InstalledVersion(pkg)
		if err != nil {
			return err
		}
		if installed == pkg.Version {
			fmt.Fprintf(r.Out, "%s is already installed at %s\n", name, installed)
			continue
		}
		if err := manager.Install(pkg); err != nil {
			return err
		}
		fmt.Fprintf(r.Out, "Installed %s %s\n", name, pkg.Version)
	}
	return nil
}

func (r Runner) runAgentUninstall(args []string) error {
	agents, options, err := parseAgentTargets(args, true)
	if err != nil {
		return err
	}
	if len(agents) == 0 {
		if !r.Interactive || r.UI == nil {
			return &UsageError{Message: "usage: ags agent uninstall <codex|claude|all> [--purge] [--yes]"}
		}
		agents, err = r.UI.SelectAgents("选择要卸载的 Agent", nil)
		if err != nil {
			return err
		}
	}
	agents, err = agent.Expand(agents)
	if err != nil {
		return err
	}
	if !options.yes {
		if !r.Interactive || r.UI == nil {
			return &UsageError{Message: "agent uninstall requires --yes in non-interactive mode"}
		}
		title := "卸载所选 Agent 并保留用户数据？"
		if options.purge {
			title = "彻底卸载所选 Agent，并删除认证、历史和配置目录？"
		}
		confirmed, err := r.UI.Confirm(title)
		if err != nil || !confirmed {
			return err
		}
	}
	service := r.environmentService()
	for _, name := range agents {
		npmName, err := agent.DefaultNPMName(name)
		if err != nil {
			return err
		}
		if err := service.Uninstall(name, agent.Package{Name: name, NPMName: npmName}, options.purge); err != nil {
			return err
		}
		fmt.Fprintf(r.Out, "Uninstalled %s\n", name)
	}
	return nil
}

func (r Runner) runAgentStatus(args []string) error {
	agents, _, err := parseAgentTargets(args, false)
	if err != nil {
		return err
	}
	if len(agents) == 0 {
		agents = agent.AllNames()
	}
	agents, err = agent.Expand(agents)
	if err != nil {
		return err
	}
	_, repo, repoErr := r.desiredRepository()
	manager := agent.Manager{Runner: r.commands()}
	for _, name := range agents {
		npmName, _ := agent.DefaultNPMName(name)
		pkg := agent.Package{Name: name, NPMName: npmName}
		desired := "-"
		if repoErr == nil {
			if locked, err := repo.AgentPackage(name); err == nil {
				pkg = locked
				desired = locked.Version
			}
		}
		installed, err := manager.InstalledVersion(pkg)
		if err != nil {
			return err
		}
		fmt.Fprintf(r.Out, "%s: installed=%s desired=%s\n", name, displayValue(installed), desired)
	}
	return nil
}

func (r Runner) runProvider(args []string) error {
	if len(args) == 0 {
		return &UsageError{Message: "usage: ags provider <switch|list|current>"}
	}
	providerRegistry, err := registry.Load(r.Paths.Registry)
	if err != nil {
		return err
	}
	service := switcher.Service{Paths: r.Paths, Registry: providerRegistry}
	switch args[0] {
	case "list":
		if len(args) != 1 {
			return &UsageError{Message: "usage: ags provider list"}
		}
		r.printProviders(providerRegistry)
		return nil
	case "current":
		if len(args) != 1 {
			return &UsageError{Message: "usage: ags provider current"}
		}
		state, err := service.Current()
		if err != nil {
			return err
		}
		fmt.Fprintf(r.Out, "codex: %s\nclaude: %s\n", currentNames(state.Codex), currentNames(state.Claude))
		return nil
	case "switch":
		return r.runProviderSwitch(args[1:], providerRegistry, service)
	default:
		return &UsageError{Message: fmt.Sprintf("unknown provider command %q", args[0])}
	}
}

func (r Runner) runProviderSwitch(args []string, providerRegistry *registry.Registry, service switcher.Service) error {
	var target switcher.Agent
	var providerName string
	var err error
	if len(args) > 0 {
		target, err = switcher.ParseAgent(args[0])
		if err != nil {
			return &UsageError{Message: err.Error()}
		}
	}
	if len(args) > 1 {
		providerName = args[1]
	}
	if len(args) > 2 {
		return &UsageError{Message: "usage: ags provider switch [codex|claude|all] [provider]"}
	}
	if target == "" || providerName == "" {
		if !r.Interactive || r.UI == nil {
			return &UsageError{Message: "usage: ags provider switch <codex|claude|all> <provider>"}
		}
		state, currentErr := service.Current()
		r.printCurrentSummary(state, currentErr)
		if target == "" {
			target, err = r.UI.SelectAgent()
			if err != nil {
				return err
			}
		}
		if providerName == "" {
			providerName, err = r.UI.SelectProvider(target, providerRegistry, state)
			if err != nil {
				return err
			}
		}
	}
	if err := service.Switch(target, providerName); err != nil {
		return err
	}
	fmt.Fprintf(r.Out, "Switched %s provider to %s\n", target, providerName)
	return nil
}

func (r Runner) runSelf(args []string) error {
	if len(args) == 0 {
		return &UsageError{Message: "usage: ags self <version|update>"}
	}
	switch args[0] {
	case "version":
		if len(args) != 1 {
			return &UsageError{Message: "usage: ags self version"}
		}
		fmt.Fprintln(r.Out, version.Version)
		return nil
	case "update":
		yes := len(args) == 2 && args[1] == "--yes"
		if len(args) > 2 || (len(args) == 2 && !yes) {
			return &UsageError{Message: "usage: ags self update [--yes]"}
		}
		if !yes {
			if !r.Interactive || r.UI == nil {
				return &UsageError{Message: "self update requires --yes in non-interactive mode"}
			}
			confirmed, err := r.UI.Confirm("下载并安装最新 AGS Release？")
			if err != nil || !confirmed {
				return err
			}
		}
		executable, err := os.Executable()
		if err != nil {
			return err
		}
		tag, err := (selfupdate.Updater{}).Update(context.Background(), executable)
		if err != nil {
			return err
		}
		fmt.Fprintf(r.Out, "Updated AGS to %s\n", tag)
		return nil
	default:
		return &UsageError{Message: fmt.Sprintf("unknown self command %q", args[0])}
	}
}

func (r Runner) runDoctor() error {
	failed := false
	for _, name := range []string{"git", "node", "npm"} {
		path, err := r.commands().LookPath(name)
		if err != nil {
			fmt.Fprintf(r.Out, "FAIL %s: not found in PATH\n", name)
			failed = true
			continue
		}
		fmt.Fprintf(r.Out, "PASS %s: %s\n", name, path)
	}
	config, err := localconfig.Load(r.Paths.AGSConfig)
	if err != nil {
		fmt.Fprintf(r.Out, "FAIL config: %v\n", err)
		failed = true
	} else {
		fmt.Fprintf(r.Out, "PASS config: %s\n", r.Paths.AGSConfig)
		if config.Environment.Source == "" {
			fmt.Fprintln(r.Out, "FAIL environment: source is not configured")
			failed = true
		} else {
			manager := environment.RepositoryManager{CacheDir: r.Paths.CacheDir, Runner: r.commands()}
			root, commit, syncErr := manager.Sync(config.Environment.Source, config.Environment.Branch)
			if syncErr != nil {
				fmt.Fprintf(r.Out, "FAIL environment: %v\n", syncErr)
				failed = true
			} else if _, loadErr := environment.LoadRepository(root); loadErr != nil {
				fmt.Fprintf(r.Out, "FAIL environment: %v\n", loadErr)
				failed = true
			} else {
				fmt.Fprintf(r.Out, "PASS environment: %s (%s)\n", root, shortCommit(commit))
			}
		}
	}
	if _, err := os.Stat(r.Paths.Registry); err == nil {
		fmt.Fprintf(r.Out, "PASS providers: %s\n", r.Paths.Registry)
	} else if errors.Is(err, os.ErrNotExist) {
		fmt.Fprintf(r.Out, "WARN providers: %s does not exist\n", r.Paths.Registry)
	} else {
		fmt.Fprintf(r.Out, "FAIL providers: %v\n", err)
		failed = true
	}
	if failed {
		return fmt.Errorf("doctor found environment problems")
	}
	return nil
}

func (r Runner) printPlan(plan environment.Plan, detailed bool) {
	fmt.Fprintf(r.Out, "Environment: source=%s profile=%s commit=%s\n", plan.Repository.Root, plan.Profile, shortCommit(plan.Commit))
	for _, item := range plan.Agents {
		fmt.Fprintf(r.Out, "%s:\n", item.Name)
		fmt.Fprintf(r.Out, "  version: %s -> %s\n", displayValue(item.InstalledVersion), item.DesiredVersion)
		if item.InstructionChanged {
			fmt.Fprintln(r.Out, "  instructions: update")
		} else {
			fmt.Fprintln(r.Out, "  instructions: unchanged")
		}
		disabledSkills := "none"
		if len(item.DisabledSkills) > 0 {
			disabledSkills = strings.Join(item.DisabledSkills, ",")
		}
		configStatus := "unchanged"
		if item.ConfigChanged {
			configStatus = "update"
		}
		if item.Name == agent.Codex {
			fmt.Fprintf(r.Out, "  disabled skills: %s (%s)\n", disabledSkills, configStatus)
		}
		counts := map[string]int{}
		for _, change := range item.SkillChanges {
			counts[change.Kind]++
		}
		fmt.Fprintf(r.Out, "  skills: +%d ~%d -%d takeover=%d\n", counts["add"], counts["update"], counts["remove"], counts["takeover"])
		if detailed {
			for _, change := range item.SkillChanges {
				fmt.Fprintf(r.Out, "    %s %s\n", change.Kind, change.Name)
			}
		}
	}
}

func (r Runner) printCurrentSummary(state switcher.CurrentState, detectionErr error) {
	fmt.Fprintln(r.Out, "Current provider:")
	if detectionErr != nil {
		fmt.Fprintf(r.Out, "  codex: unknown\n  claude: unknown\n  warning: %v\n\n", detectionErr)
		return
	}
	fmt.Fprintf(r.Out, "  codex: %s\n  claude: %s\n\n", currentNames(state.Codex), currentNames(state.Claude))
}

func (r Runner) printUsage() {
	fmt.Fprint(r.Out, `AGS manages Codex and Claude Code environments.

Usage:
  ags
  ags env source set <git-url-or-path> [--branch main]
  ags env configure [--profile name] [--agents codex,claude]
  ags env apply [--profile name] [--agents codex,claude] [--yes]
  ags env status
  ags env diff
  ags env lock --repo <path>
  ags env vendor --repo <path>
  ags env validate --repo <path>
  ags agent install <codex|claude|all>
  ags agent uninstall <codex|claude|all> [--purge] [--yes]
  ags agent status [codex|claude|all]
  ags provider switch <codex|claude|all> <provider>
  ags provider list
  ags provider current
  ags self version
  ags self update [--yes]
  ags doctor

Configuration:
  ~/.ags/config.yaml
  ~/.ags/providers.yaml
`)
}

func (r Runner) printProviders(providerRegistry *registry.Registry) {
	w := tabwriter.NewWriter(r.Out, 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "PROVIDER\tCONFIG MODE\tCODEX MODEL\tCODEX BASE URL\tCLAUDE MODEL\tCLAUDE BASE URL")
	for _, name := range providerRegistry.Names() {
		provider, err := providerRegistry.Provider(name)
		if err != nil {
			continue
		}
		mode := providerRegistry.Providers[name].ConfigMode()
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n", name, mode, codexModel(provider), codexBaseURL(provider), claudeModel(provider), claudeBaseURL(provider))
	}
	_ = w.Flush()
}

type uninstallOptions struct {
	purge bool
	yes   bool
}

func parseAgentTargets(args []string, uninstall bool) ([]agent.Name, uninstallOptions, error) {
	var targets []agent.Name
	options := uninstallOptions{}
	for _, value := range args {
		switch value {
		case "--purge":
			if !uninstall {
				return nil, options, &UsageError{Message: "--purge is only valid for uninstall"}
			}
			options.purge = true
		case "--yes":
			options.yes = true
		default:
			name, err := agent.Parse(value, true)
			if err != nil {
				return nil, options, &UsageError{Message: err.Error()}
			}
			targets = append(targets, name)
		}
	}
	return targets, options, nil
}

func parseEnvironmentOptions(args []string) (string, []agent.Name, bool, error) {
	var profile string
	var agents []agent.Name
	yes := false
	for index := 0; index < len(args); index++ {
		switch args[index] {
		case "--profile":
			if index+1 >= len(args) {
				return "", nil, false, &UsageError{Message: "--profile requires a value"}
			}
			profile = args[index+1]
			index++
		case "--agents":
			if index+1 >= len(args) {
				return "", nil, false, &UsageError{Message: "--agents requires a value"}
			}
			for _, value := range strings.Split(args[index+1], ",") {
				name, err := agent.Parse(value, true)
				if err != nil {
					return "", nil, false, &UsageError{Message: err.Error()}
				}
				agents = append(agents, name)
			}
			index++
		case "--yes":
			yes = true
		default:
			return "", nil, false, &UsageError{Message: fmt.Sprintf("unknown option %q", args[index])}
		}
	}
	expanded, err := agent.Expand(agents)
	if err != nil {
		return "", nil, false, err
	}
	return profile, expanded, yes, nil
}

func repoOption(args []string) (string, error) {
	if len(args) == 0 {
		return ".", nil
	}
	if len(args) == 2 && args[0] == "--repo" {
		return args[1], nil
	}
	return "", &UsageError{Message: "usage: ags env <lock|vendor|validate> [--repo <path>]"}
}

func joinAgents(names []agent.Name) string {
	values := make([]string, len(names))
	for index, name := range names {
		values[index] = string(name)
	}
	sort.Strings(values)
	return strings.Join(values, ",")
}

func shortCommit(value string) string {
	if len(value) > 12 {
		return value[:12]
	}
	return value
}

func displayValue(value string) string {
	if strings.TrimSpace(value) == "" {
		return "-"
	}
	return value
}

func currentNames(names []string) string {
	if len(names) == 0 {
		return "unknown"
	}
	return strings.Join(names, ", ")
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

func claudeModel(provider registry.Provider) string {
	if provider.Claude != nil && provider.Claude.Model != "" {
		return provider.Claude.Model
	}
	return "-"
}

func claudeBaseURL(provider registry.Provider) string {
	if provider.Claude != nil {
		return provider.Claude.BaseURL
	}
	return "-"
}

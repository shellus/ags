package environment

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shellus/ags/internal/agent"
	"github.com/shellus/ags/internal/configfile"
	"github.com/shellus/ags/internal/localconfig"
)

type serviceRunner struct {
	npmRoot string
}

func (r *serviceRunner) Run(_ string, _ []string, name string, args ...string) ([]byte, error) {
	if name == "git" {
		return nil, fmt.Errorf("not a git checkout")
	}
	if name != "npm" {
		return nil, nil
	}
	if len(args) >= 2 && args[0] == "root" && args[1] == "-g" {
		return []byte(r.npmRoot + "\n"), nil
	}
	if len(args) >= 3 && args[0] == "install" && args[1] == "-g" {
		value := args[2]
		index := strings.LastIndex(value, "@")
		if index <= 0 {
			return nil, fmt.Errorf("invalid package %s", value)
		}
		packageName, version := value[:index], value[index+1:]
		packageDir := filepath.Join(append([]string{r.npmRoot}, strings.Split(packageName, "/")...)...)
		if err := os.MkdirAll(packageDir, 0o755); err != nil {
			return nil, err
		}
		data, _ := json.Marshal(map[string]string{"version": version})
		return nil, os.WriteFile(filepath.Join(packageDir, "package.json"), data, 0o644)
	}
	if len(args) >= 3 && args[0] == "uninstall" && args[1] == "-g" {
		packageDir := filepath.Join(append([]string{r.npmRoot}, strings.Split(args[2], "/")...)...)
		return nil, os.RemoveAll(packageDir)
	}
	return nil, nil
}

func (r *serviceRunner) LookPath(name string) (string, error) { return name, nil }

func TestServicePrepareAndApply(t *testing.T) {
	root := t.TempDir()
	repoRoot := filepath.Join(root, "repo")
	writeTestFile(t, filepath.Join(repoRoot, "instructions", "global.md"), "shared rules\n")
	writeTestFile(t, filepath.Join(repoRoot, "skills", "local", "demo", "SKILL.md"), "---\nname: demo\ndescription: demo\n---\n")
	writeTestFile(t, filepath.Join(repoRoot, "environment.yaml"), `version: 1
instructions: {global: instructions/global.md}
agents:
  codex: {package: "@openai/codex"}
  claude: {package: "@anthropic-ai/claude-code"}
sources:
  local:
    type: local
    path: skills/local
    discover: {mode: flat}
profiles:
  default:
    include: ["local:*"]
    agents:
      codex: {preserve: [".system"]}
      claude: {}
`)
	writeTestFile(t, filepath.Join(repoRoot, "environment.lock"), `version: 1
agents:
  codex: {version: "1.2.3"}
  claude: {version: "2.3.4"}
sources: {}
`)
	paths := configfile.Paths{
		CacheDir:       filepath.Join(root, "cache"),
		StateDir:       filepath.Join(root, "state"),
		CodexDir:       filepath.Join(root, ".codex"),
		CodexGuidance:  filepath.Join(root, ".codex", "AGENTS.md"),
		CodexSkills:    filepath.Join(root, ".codex", "skills"),
		ClaudeDir:      filepath.Join(root, ".claude"),
		ClaudeGuidance: filepath.Join(root, ".claude", "CLAUDE.md"),
		ClaudeSkills:   filepath.Join(root, ".claude", "skills"),
	}
	runner := &serviceRunner{npmRoot: filepath.Join(root, "npm")}
	service := Service{Paths: paths, Runner: runner}
	config := localconfig.Config{
		Version: localconfig.CurrentVersion,
		Environment: localconfig.EnvironmentConfig{
			Source:  repoRoot,
			Branch:  "main",
			Profile: "default",
			Agents:  []agent.Name{agent.Codex, agent.Claude},
		},
	}
	plan, err := service.Prepare(config, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	defer plan.Cleanup()
	if !plan.HasChanges() || plan.HasConflicts() {
		t.Fatalf("unexpected plan: %#v", plan)
	}
	if err := service.Apply(plan); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{
		paths.CodexGuidance,
		paths.ClaudeGuidance,
		filepath.Join(paths.CodexSkills, "demo", "SKILL.md"),
		filepath.Join(paths.ClaudeSkills, "demo", "SKILL.md"),
		filepath.Join(paths.StateDir, "environment.json"),
	} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("missing applied path %s: %v", path, err)
		}
	}
	second, err := service.Prepare(config, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	defer second.Cleanup()
	if second.HasChanges() {
		t.Fatalf("second plan still has changes: %#v", second.Agents)
	}
}

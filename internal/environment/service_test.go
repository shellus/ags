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
	writeTestFile(t, filepath.Join(repoRoot, "skills", "vendor", snapshotFilename), `version: 1
sources:
  upstream: {commit: aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa}
include: [upstream]
skills: {}
`)
	writeTestFile(t, filepath.Join(repoRoot, "environment.yaml"), `version: 2
instructions: {global: instructions/global.md}
skills:
  local: skills/local
  vendor: skills/vendor
  vendor_include: [upstream]
agents:
  codex: {package: "@openai/codex"}
  claude: {package: "@anthropic-ai/claude-code"}
sources:
  upstream:
    type: git
    url: git@example.com:upstream/skills.git
    discover: {mode: single, path: skill}
profiles:
  default:
    include: ["local:*"]
    agents:
      codex: {preserve: [".system"]}
      claude: {}
`)
	writeTestFile(t, filepath.Join(repoRoot, "environment.lock"), `version: 2
agents:
  codex: {version: "1.2.3"}
  claude: {version: "2.3.4"}
sources:
  upstream: {commit: aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa}
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
	writeTestFile(t, filepath.Join(paths.CodexSkills, "demo", "SKILL.md"), "manual version\n")
	writeTestFile(t, filepath.Join(paths.CodexSkills, "unmanaged", "SKILL.md"), "keep me\n")
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
	if !plan.HasChanges() {
		t.Fatalf("unexpected plan: %#v", plan)
	}
	var codexPlan *AgentPlan
	for index := range plan.Agents {
		if plan.Agents[index].Name == agent.Codex {
			codexPlan = &plan.Agents[index]
			break
		}
	}
	if codexPlan == nil || len(codexPlan.SkillChanges) != 1 || codexPlan.SkillChanges[0].Kind != "takeover" {
		t.Fatalf("Codex takeover plan = %#v", plan.Agents)
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
	content, err := os.ReadFile(filepath.Join(paths.CodexSkills, "demo", "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "---\nname: demo\ndescription: demo\n---\n" {
		t.Fatalf("Codex demo was not replaced: %q", content)
	}
	if content, err := os.ReadFile(filepath.Join(paths.CodexSkills, "unmanaged", "SKILL.md")); err != nil || string(content) != "keep me\n" {
		t.Fatalf("unrelated unmanaged Skill changed: %q, %v", content, err)
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

func TestRestoreManagedRestoresTakeoverAfterPartialApply(t *testing.T) {
	root := t.TempDir()
	paths := configfile.Paths{
		StateDir:      filepath.Join(root, "state"),
		CodexGuidance: filepath.Join(root, ".codex", "AGENTS.md"),
		CodexSkills:   filepath.Join(root, ".codex", "skills"),
	}
	writeTestFile(t, paths.CodexGuidance, "old rules\n")
	writeTestFile(t, filepath.Join(paths.CodexSkills, "demo", "SKILL.md"), "old demo\n")
	writeTestFile(t, filepath.Join(paths.CodexSkills, "unmanaged", "SKILL.md"), "keep me\n")

	plan := Plan{
		Build: BuildResult{Root: filepath.Join(root, "build"), Instruction: []byte("new rules\n")},
		Agents: []AgentPlan{{
			Name:                   agent.Codex,
			InstructionDestination: paths.CodexGuidance,
			SkillsDestination:      paths.CodexSkills,
			DesiredSkills:          map[string]string{"demo": "new", "added": "new"},
			ManagedBefore:          map[string]string{},
			TakeoverBefore:         map[string]string{"demo": "old"},
		}},
	}
	if err := os.MkdirAll(plan.Build.Root, 0o755); err != nil {
		t.Fatal(err)
	}
	service := Service{Paths: paths}
	backups, err := service.backupManaged(plan)
	if err != nil {
		t.Fatal(err)
	}

	writeTestFile(t, paths.CodexGuidance, "new rules\n")
	if err := os.RemoveAll(filepath.Join(paths.CodexSkills, "demo")); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(paths.CodexSkills, "demo", "SKILL.md"), "new demo\n")
	writeTestFile(t, filepath.Join(paths.CodexSkills, "added", "SKILL.md"), "partially added\n")
	if err := writeMarker(paths.CodexSkills, plan.Agents[0].DesiredSkills); err != nil {
		t.Fatal(err)
	}
	if err := service.restoreManaged(backups); err != nil {
		t.Fatal(err)
	}

	for path, expected := range map[string]string{
		paths.CodexGuidance: "old rules\n",
		filepath.Join(paths.CodexSkills, "demo", "SKILL.md"):      "old demo\n",
		filepath.Join(paths.CodexSkills, "unmanaged", "SKILL.md"): "keep me\n",
	} {
		content, err := os.ReadFile(path)
		if err != nil || string(content) != expected {
			t.Fatalf("restored %s = %q, %v", path, content, err)
		}
	}
	for _, path := range []string{
		filepath.Join(paths.CodexSkills, "added"),
		filepath.Join(paths.CodexSkills, ".ags-managed.json"),
	} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("partial apply path still exists: %s", path)
		}
	}
}

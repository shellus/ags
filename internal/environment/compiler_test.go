package environment

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/shellus/ags/internal/agent"
)

func TestCompilerBuildsLocalProfile(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "instructions", "global.md"), "global rules\n")
	writeTestFile(t, filepath.Join(root, "skills", "local", "demo", "SKILL.md"), "---\nname: demo\ndescription: demo\n---\n")
	writeTestFile(t, filepath.Join(root, "skills", "vendor", snapshotFilename), "version: 1\nsources: {}\nskills: {}\n")
	writeTestFile(t, filepath.Join(root, "environment.yaml"), `version: 2
instructions: {global: instructions/global.md}
skills: {local: skills/local, vendor: skills/vendor}
agents:
  codex: {package: "@openai/codex"}
  claude: {package: "@anthropic-ai/claude-code"}
sources: {}
profiles:
  default:
    include: ["*"]
    agents:
      codex: {preserve: [".system"]}
      claude: {}
`)
	writeTestFile(t, filepath.Join(root, "environment.lock"), `version: 2
agents:
  codex: {version: "1.0.0"}
  claude: {version: "2.0.0"}
sources: {}
`)
	repo, err := LoadRepository(root)
	if err != nil {
		t.Fatal(err)
	}
	compiler := Compiler{CacheDir: filepath.Join(t.TempDir(), "cache")}
	result, err := compiler.Build(repo, "default", []agent.Name{agent.Codex})
	if err != nil {
		t.Fatal(err)
	}
	defer result.Cleanup()
	if string(result.Instruction) != "global rules\n" {
		t.Fatalf("instruction = %q", result.Instruction)
	}
	if _, err := os.Stat(filepath.Join(result.Agents[agent.Codex].Stage, "demo", "SKILL.md")); err != nil {
		t.Fatal(err)
	}
}

func TestCompilerVendorsProcessedSourcesIntoRepository(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "instructions", "global.md"), "rules\n")
	writeTestFile(t, filepath.Join(root, "skills", "local", "local-skill", "SKILL.md"), "---\nname: local-skill\n---\n")
	writeTestFile(t, filepath.Join(root, "upstream", "SKILL.md"), "---\nname: external-skill\n---\n")
	writeTestFile(t, filepath.Join(root, "environment.yaml"), `version: 2
instructions: {global: instructions/global.md}
skills:
  local: skills/local
  vendor: skills/vendor
  vendor_include: [external]
agents:
  codex: {package: "@openai/codex"}
  claude: {package: "@anthropic-ai/claude-code"}
sources:
  external:
    type: local
    path: upstream
    discover: {mode: single}
profiles:
  default:
    include: ["*"]
    agents:
      codex: {}
      claude: {}
`)
	writeTestFile(t, filepath.Join(root, "environment.lock"), `version: 2
agents:
  codex: {version: "1.0.0"}
  claude: {version: "2.0.0"}
sources: {}
`)
	repo, err := LoadRepository(root)
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := (Compiler{CacheDir: filepath.Join(t.TempDir(), "cache")}).Vendor(repo)
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Skills["external-skill"].Source != "external" {
		t.Fatalf("snapshot = %#v", snapshot)
	}
	if err := ValidateRepository(root); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "skills", "vendor", "external-skill", "SKILL.md")); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(root, "environment.lock"), `version: 2
agents:
  codex: {version: "1.0.0"}
  claude: {version: "2.0.0"}
sources:
  stale: {commit: aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa}
`)
	staleRepo, err := LoadRepository(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := ValidatePublishedSkills(staleRepo); err == nil {
		t.Fatal("stale Skill snapshot was accepted")
	}
}

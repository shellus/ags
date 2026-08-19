package environment

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/shellus/ags/internal/agent"
)

func TestLoadRepositoryAndSelection(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "instructions", "global.md"), "rules")
	writeTestFile(t, filepath.Join(root, "skills", "local", "demo", "SKILL.md"), "---\nname: demo\ndescription: demo\n---\n")
	writeTestFile(t, filepath.Join(root, "skills", "vendor", snapshotFilename), "version: 1\nsources: {}\nskills: {}\n")
	writeTestFile(t, filepath.Join(root, "environment.yaml"), `version: 2
instructions:
  global: instructions/global.md
skills:
  local: skills/local
  vendor: skills/vendor
agents:
  codex: {package: "@openai/codex"}
  claude: {package: "@anthropic-ai/claude-code"}
sources: {}
profiles:
  default:
    include: ["*"]
    agents:
      codex: {disabled_skills: ["openai-docs"]}
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
	profile, include, _, err := repo.Selection("default", agent.Codex)
	if err != nil || len(include) != 1 || len(profile.DisabledSkills) != 1 || profile.DisabledSkills[0] != "openai-docs" {
		t.Fatalf("Selection() = %#v %#v %v", profile, include, err)
	}
}

func TestRepositoryRejectsDisabledSkillsForClaude(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "instructions", "global.md"), "rules")
	writeTestFile(t, filepath.Join(root, "skills", "local", "demo", "SKILL.md"), "---\nname: demo\n---\n")
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
      codex: {}
      claude: {disabled_skills: ["openai-docs"]}
`)
	writeTestFile(t, filepath.Join(root, "environment.lock"), `version: 2
agents:
  codex: {version: "1.0.0"}
  claude: {version: "2.0.0"}
sources: {}
`)
	if _, err := LoadRepository(root); err == nil {
		t.Fatal("Claude disabled_skills was accepted")
	}
}

func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

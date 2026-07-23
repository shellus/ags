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
	writeTestFile(t, filepath.Join(root, "environment.yaml"), `version: 1
instructions:
  global: instructions/global.md
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
	writeTestFile(t, filepath.Join(root, "environment.lock"), `version: 1
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
	if err != nil || len(include) != 1 || len(profile.Preserve) != 1 {
		t.Fatalf("Selection() = %#v %#v %v", profile, include, err)
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

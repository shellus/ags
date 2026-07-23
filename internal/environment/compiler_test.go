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
	writeTestFile(t, filepath.Join(root, "environment.yaml"), `version: 1
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

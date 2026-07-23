package environment

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type lockRunner struct {
	gitCalls int
}

func (r *lockRunner) Run(_ string, _ []string, name string, args ...string) ([]byte, error) {
	if name == "npm" && len(args) >= 3 && args[0] == "view" {
		if strings.Contains(args[1], "openai") {
			return []byte("3.0.0\n"), nil
		}
		return []byte("4.0.0\n"), nil
	}
	if name == "git" && len(args) >= 3 && args[0] == "ls-remote" {
		r.gitCalls++
		return []byte("0123456789abcdef0123456789abcdef01234567\tHEAD\n"), nil
	}
	return nil, nil
}

func (r *lockRunner) LookPath(name string) (string, error) { return name, nil }

func TestUpdateLockResolvesAgentsAndSharedSourceOnce(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "instructions", "global.md"), "rules")
	writeTestFile(t, filepath.Join(root, "environment.yaml"), `version: 1
instructions: {global: instructions/global.md}
agents:
  codex: {package: "@openai/codex"}
  claude: {package: "@anthropic-ai/claude-code"}
sources:
  one:
    type: git
    url: https://example.com/shared.git
    lock: shared
    discover: {mode: single, path: skill}
  two:
    type: git
    url: https://example.com/shared.git
    lock: shared
    discover: {mode: collection, path: skills}
profiles:
  default:
    include: [one]
    agents:
      codex: {}
      claude: {}
`)
	writeTestFile(t, filepath.Join(root, "environment.lock"), `version: 1
agents:
  codex: {version: "1.0.0"}
  claude: {version: "2.0.0"}
sources:
  shared: {commit: aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa}
`)
	runner := &lockRunner{}
	lock, err := UpdateLock(root, runner)
	if err != nil {
		t.Fatal(err)
	}
	if lock.Agents["codex"].Version != "3.0.0" || lock.Agents["claude"].Version != "4.0.0" {
		t.Fatalf("agent locks = %#v", lock.Agents)
	}
	if runner.gitCalls != 1 {
		t.Fatalf("git ls-remote calls = %d", runner.gitCalls)
	}
	if _, err := os.Stat(filepath.Join(root, "environment.lock")); err != nil {
		t.Fatal(err)
	}
}

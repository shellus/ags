package environment

import (
	"os"
	"path/filepath"
	"strings"
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
	if got := result.Agents[agent.Codex].DisabledSkills; len(got) != 1 || got[0] != "openai-docs" {
		t.Fatalf("disabled Skills = %#v", got)
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
	vendoredSkill := filepath.Join(root, "skills", "vendor", "external-skill", "SKILL.md")
	if _, err := os.Stat(vendoredSkill); err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(vendoredSkill)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(vendoredSkill, []byte(strings.ReplaceAll(string(content), "\n", "\r\n")), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := ValidateRepository(root); err != nil {
		t.Fatalf("CRLF checkout rejected: %v", err)
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

func TestTreeHashNormalizesTextLineEndings(t *testing.T) {
	lfRoot := t.TempDir()
	crlfRoot := t.TempDir()
	writeTestFile(t, filepath.Join(lfRoot, "SKILL.md"), "line one\nline two\n")
	writeTestFile(t, filepath.Join(crlfRoot, "SKILL.md"), "line one\r\nline two\r\n")

	lfHash, err := TreeHash(lfRoot)
	if err != nil {
		t.Fatal(err)
	}
	crlfHash, err := TreeHash(crlfRoot)
	if err != nil {
		t.Fatal(err)
	}
	if lfHash != crlfHash {
		t.Fatalf("LF hash %s differs from CRLF hash %s", lfHash, crlfHash)
	}
}

func TestTreeHashDoesNotNormalizeBinaryContent(t *testing.T) {
	firstRoot := t.TempDir()
	secondRoot := t.TempDir()
	if err := os.WriteFile(filepath.Join(firstRoot, "asset.bin"), []byte{0, '\r', '\n'}, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(secondRoot, "asset.bin"), []byte{0, '\n'}, 0o644); err != nil {
		t.Fatal(err)
	}

	firstHash, err := TreeHash(firstRoot)
	if err != nil {
		t.Fatal(err)
	}
	secondHash, err := TreeHash(secondRoot)
	if err != nil {
		t.Fatal(err)
	}
	if firstHash == secondHash {
		t.Fatal("binary CRLF bytes were normalized")
	}
}

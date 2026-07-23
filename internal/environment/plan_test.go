package environment

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/shellus/ags/internal/agent"
	"github.com/shellus/ags/internal/configfile"
	"github.com/shellus/ags/internal/localconfig"
)

func TestPreparePlanPlansUnmanagedTakeover(t *testing.T) {
	home := t.TempDir()
	paths := configfile.Paths{
		StateDir:       filepath.Join(home, "state"),
		CodexGuidance:  filepath.Join(home, ".codex", "AGENTS.md"),
		CodexSkills:    filepath.Join(home, ".codex", "skills"),
		ClaudeGuidance: filepath.Join(home, ".claude", "CLAUDE.md"),
		ClaudeSkills:   filepath.Join(home, ".claude", "skills"),
	}
	stage := filepath.Join(home, "stage", "codex")
	writeTestFile(t, filepath.Join(stage, "demo", "SKILL.md"), "desired")
	writeTestFile(t, filepath.Join(paths.CodexSkills, "demo", "SKILL.md"), "manual")
	repo := Repository{
		Manifest: Manifest{Agents: map[agent.Name]Agent{agent.Codex: {Package: "@openai/codex"}}},
		Lock:     Lock{Agents: map[agent.Name]AgentLock{agent.Codex: {Version: "1.0.0"}}},
	}
	build := BuildResult{Root: filepath.Join(home, "stage"), Instruction: []byte("rules"), Agents: map[agent.Name]RenderedAgent{
		agent.Codex: {Name: agent.Codex, Stage: stage, Skills: []string{"demo"}},
	}}
	plan, err := preparePlan(paths, localconfig.Default(), "source", "main", "commit", "default", repo, build, map[agent.Name]string{agent.Codex: "1.0.0"})
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Agents) != 1 {
		t.Fatalf("plan = %#v", plan.Agents)
	}
	item := plan.Agents[0]
	if item.TakeoverBefore["demo"] == "" {
		t.Fatalf("takeover = %#v", item.TakeoverBefore)
	}
	if len(item.SkillChanges) != 1 || item.SkillChanges[0] != (SkillChange{Name: "demo", Kind: "takeover"}) {
		t.Fatalf("changes = %#v", item.SkillChanges)
	}
	if !plan.HasChanges() {
		t.Fatal("takeover plan has no changes")
	}
	_ = os.RemoveAll(build.Root)
}

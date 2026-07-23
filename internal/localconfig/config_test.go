package localconfig

import (
	"path/filepath"
	"testing"

	"github.com/shellus/ags/internal/agent"
)

func TestSaveAndLoad(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ags", "config.yaml")
	want := Config{
		Version: CurrentVersion,
		Environment: EnvironmentConfig{
			Source:  "git@github.com:shellus/agent-env.git",
			Branch:  "main",
			Profile: "default",
			Agents:  []agent.Name{agent.Claude, agent.Codex},
		},
	}
	if err := Save(path, want); err != nil {
		t.Fatal(err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.Environment.Source != want.Environment.Source || len(got.Environment.Agents) != 2 {
		t.Fatalf("Load() = %#v", got)
	}
}

package agent

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

type fakeRunner struct {
	root  string
	calls [][]string
}

func (r *fakeRunner) Run(_ string, _ []string, name string, args ...string) ([]byte, error) {
	r.calls = append(r.calls, append([]string{name}, args...))
	if len(args) >= 2 && args[0] == "root" {
		return []byte(r.root + "\n"), nil
	}
	if len(args) >= 3 && args[0] == "view" {
		return []byte("9.9.9\n"), nil
	}
	return nil, nil
}

func (r *fakeRunner) LookPath(name string) (string, error) { return name, nil }

func TestManagerInstalledAndInstall(t *testing.T) {
	root := t.TempDir()
	packageDir := filepath.Join(root, "@openai", "codex")
	if err := os.MkdirAll(packageDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(packageDir, "package.json"), []byte(`{"version":"1.2.3"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	runner := &fakeRunner{root: root}
	manager := Manager{Runner: runner}
	pkg := Package{Name: Codex, NPMName: "@openai/codex", Version: "1.2.4"}
	version, err := manager.InstalledVersion(pkg)
	if err != nil || version != "1.2.3" {
		t.Fatalf("InstalledVersion() = %q, %v", version, err)
	}
	if err := manager.Install(pkg); err != nil {
		t.Fatal(err)
	}
	want := []string{"npm", "install", "-g", "@openai/codex@1.2.4"}
	if !reflect.DeepEqual(runner.calls[len(runner.calls)-1], want) {
		t.Fatalf("install call = %#v", runner.calls)
	}
}

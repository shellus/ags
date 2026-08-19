package transaction

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestApplyReplacesExistingFileAndPreservesMode(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte("old"), 0o640); err != nil {
		t.Fatal(err)
	}

	if err := Apply([]Change{{Path: path, Content: []byte("new")}}); err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "new" {
		t.Fatalf("content = %q, want new", content)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	assertMode(t, info, 0o640)
}

func TestApplyCreatesPrivateFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "settings.json")
	if err := Apply([]Change{{Path: path, Content: []byte("{}\n")}}); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	assertMode(t, info, 0o600)
}

func TestApplyUpdatesSymbolicLinkTargetWithoutReplacingLink(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "shared-config.json")
	link := filepath.Join(dir, "config.json")
	if err := os.WriteFile(target, []byte("old"), 0o640); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("create symbolic link: %v", err)
	}

	if err := Apply([]Change{{Path: link, Content: []byte("new")}}); err != nil {
		t.Fatal(err)
	}
	info, err := os.Lstat(link)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Fatal("symbolic link was replaced")
	}
	content, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "new" {
		t.Fatalf("target content = %q, want new", content)
	}
	targetInfo, err := os.Stat(target)
	if err != nil {
		t.Fatal(err)
	}
	assertMode(t, targetInfo, 0o640)
}

func assertMode(t *testing.T, info os.FileInfo, expected os.FileMode) {
	t.Helper()
	if runtime.GOOS == "windows" {
		return
	}
	if info.Mode().Perm() != expected {
		t.Fatalf("mode = %o, want %o", info.Mode().Perm(), expected)
	}
}

func TestApplyRejectsDuplicatePathsBeforeWriting(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}

	err := Apply([]Change{
		{Path: path, Content: []byte("first")},
		{Path: path, Content: []byte("second")},
	})
	if err == nil {
		t.Fatal("Apply() succeeded with duplicate paths")
	}
	content, readErr := os.ReadFile(path)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(content) != "old" {
		t.Fatalf("content changed to %q after validation failure", content)
	}
}

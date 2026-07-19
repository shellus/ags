package transaction

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

type Change struct {
	Path    string
	Content []byte
}

type snapshot struct {
	path    string
	content []byte
	mode    os.FileMode
	existed bool
}

func Apply(changes []Change) error {
	if len(changes) == 0 {
		return nil
	}

	seen := make(map[string]struct{}, len(changes))
	snapshots := make([]snapshot, 0, len(changes))
	pending := make([]Change, 0, len(changes))

	for _, change := range changes {
		if change.Path == "" {
			return fmt.Errorf("change path must not be empty")
		}
		if _, ok := seen[change.Path]; ok {
			return fmt.Errorf("duplicate change path: %s", change.Path)
		}
		seen[change.Path] = struct{}{}

		info, err := os.Stat(change.Path)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("stat %s: %w", change.Path, err)
		}

		original, readErr := os.ReadFile(change.Path)
		if readErr != nil && !errors.Is(readErr, os.ErrNotExist) {
			return fmt.Errorf("read %s: %w", change.Path, readErr)
		}

		item := snapshot{path: change.Path, content: original, mode: 0o600}
		if err == nil {
			item.existed = true
			item.mode = info.Mode().Perm()
		}
		snapshots = append(snapshots, item)

		if item.existed && bytes.Equal(item.content, change.Content) {
			continue
		}
		pending = append(pending, change)
	}

	committed := make([]snapshot, 0, len(pending))
	for _, change := range pending {
		before := findSnapshot(snapshots, change.Path)
		if err := writeFile(change.Path, change.Content, before.mode); err != nil {
			rollbackErr := rollback(committed)
			if rollbackErr != nil {
				return errors.Join(fmt.Errorf("write %s: %w", change.Path, err), fmt.Errorf("rollback: %w", rollbackErr))
			}
			return fmt.Errorf("write %s: %w", change.Path, err)
		}
		committed = append(committed, before)
	}
	return nil
}

func findSnapshot(snapshots []snapshot, path string) snapshot {
	for _, item := range snapshots {
		if item.path == path {
			return item
		}
	}
	panic("missing transaction snapshot")
}

func rollback(committed []snapshot) error {
	var rollbackErr error
	for i := len(committed) - 1; i >= 0; i-- {
		item := committed[i]
		if !item.existed {
			if err := os.Remove(item.path); err != nil && !errors.Is(err, os.ErrNotExist) {
				rollbackErr = errors.Join(rollbackErr, fmt.Errorf("remove %s: %w", item.path, err))
			}
			continue
		}
		if err := writeFile(item.path, item.content, item.mode); err != nil {
			rollbackErr = errors.Join(rollbackErr, fmt.Errorf("restore %s: %w", item.path, err))
		}
	}
	return rollbackErr
}

func writeFile(path string, content []byte, mode os.FileMode) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}

	tmp, err := os.CreateTemp(dir, "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	removeTemp := true
	defer func() {
		if removeTemp {
			_ = os.Remove(tmpPath)
		}
	}()

	if err := tmp.Chmod(mode.Perm()); err != nil {
		_ = tmp.Close()
		return err
	}
	if _, err := tmp.Write(content); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}

	if err := replace(tmpPath, path); err != nil {
		return err
	}
	removeTemp = false
	return nil
}

func replace(source, target string) error {
	if runtime.GOOS != "windows" {
		return os.Rename(source, target)
	}

	backup := target + ".ags-backup"
	_ = os.Remove(backup)

	_, err := os.Stat(target)
	if errors.Is(err, os.ErrNotExist) {
		return os.Rename(source, target)
	}
	if err != nil {
		return err
	}
	if err := os.Rename(target, backup); err != nil {
		return err
	}
	if err := os.Rename(source, target); err != nil {
		_ = os.Rename(backup, target)
		return err
	}
	_ = os.Remove(backup)
	return nil
}

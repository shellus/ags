package environment

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"

	"gopkg.in/yaml.v3"
)

const snapshotVersion = 1
const snapshotFilename = ".ags-snapshot.yaml"

type SkillSnapshot struct {
	Version int                      `yaml:"version"`
	Sources map[string]SourceLock    `yaml:"sources"`
	Include []string                 `yaml:"include"`
	Skills  map[string]SnapshotSkill `yaml:"skills"`
}

type SnapshotSkill struct {
	Source string `yaml:"source"`
	Hash   string `yaml:"hash"`
}

func publishedUnits(repo Repository) (map[string][]SkillUnit, error) {
	resolved := map[string][]SkillUnit{}
	for name, relative := range map[string]string{
		"local":  repo.Manifest.Skills.Local,
		"vendor": repo.Manifest.Skills.Vendor,
	} {
		root := filepath.Join(repo.Root, filepath.FromSlash(relative))
		units, err := discoverUnits(name, root, Discover{Mode: "flat"})
		if err != nil {
			return nil, err
		}
		resolved[name] = units
	}
	return resolved, nil
}

func ValidatePublishedSkills(repo Repository) error {
	resolved, err := publishedUnits(repo)
	if err != nil {
		return err
	}
	seen := map[string]string{}
	for source, units := range resolved {
		for _, unit := range units {
			if previous := seen[unit.Name]; previous != "" {
				return fmt.Errorf("published Skill %q exists in both %s and %s", unit.Name, previous, source)
			}
			seen[unit.Name] = source
		}
	}

	path := filepath.Join(repo.Root, filepath.FromSlash(repo.Manifest.Skills.Vendor), snapshotFilename)
	var snapshot SkillSnapshot
	if err := loadYAML(path, &snapshot); err != nil {
		return err
	}
	if snapshot.Version != snapshotVersion {
		return fmt.Errorf("Skill snapshot version must be %d", snapshotVersion)
	}
	if err := compareSnapshotSources(repo.Lock.Sources, snapshot.Sources); err != nil {
		return err
	}
	if !slices.Equal(snapshot.Include, repo.Manifest.Skills.VendorInclude) {
		return fmt.Errorf("Skill snapshot include selection does not match environment.yaml")
	}
	if len(snapshot.Skills) != len(resolved["vendor"]) {
		return fmt.Errorf("Skill snapshot records %d vendored Skills, repository contains %d", len(snapshot.Skills), len(resolved["vendor"]))
	}
	for _, unit := range resolved["vendor"] {
		record, ok := snapshot.Skills[unit.Name]
		if !ok {
			return fmt.Errorf("vendored Skill %s is missing from %s", unit.Name, snapshotFilename)
		}
		if record.Source == "" {
			return fmt.Errorf("vendored Skill %s has no upstream source", unit.Name)
		}
		hash, err := TreeHash(unit.Path)
		if err != nil {
			return err
		}
		if record.Hash != hash {
			return fmt.Errorf("vendored Skill %s does not match its published hash", unit.Name)
		}
	}
	return nil
}

func compareSnapshotSources(expected, actual map[string]SourceLock) error {
	if len(expected) != len(actual) {
		return fmt.Errorf("Skill snapshot source lock count is %d, expected %d", len(actual), len(expected))
	}
	for name, lock := range expected {
		if actual[name].Commit != lock.Commit {
			return fmt.Errorf("Skill snapshot source %s is %s, expected %s", name, actual[name].Commit, lock.Commit)
		}
	}
	return nil
}

func (c Compiler) Vendor(repo Repository) (SkillSnapshot, error) {
	workRoot, err := c.tempDir("vendor-*")
	if err != nil {
		return SkillSnapshot{}, err
	}
	defer os.RemoveAll(workRoot)

	resolved, err := c.resolveSources(repo, filepath.Join(workRoot, "sources"))
	if err != nil {
		return SkillSnapshot{}, err
	}
	selected, err := selectUnits(repo.Manifest.Skills.VendorInclude, nil, resolved)
	if err != nil {
		return SkillSnapshot{}, fmt.Errorf("select vendored Skills: %w", err)
	}
	localRoot := filepath.Join(repo.Root, filepath.FromSlash(repo.Manifest.Skills.Local))
	localUnits, err := discoverUnits("local", localRoot, Discover{Mode: "flat"})
	if err != nil {
		return SkillSnapshot{}, err
	}
	seen := map[string]string{}
	for _, unit := range localUnits {
		seen[unit.Name] = "local"
	}

	vendorRoot := filepath.Join(repo.Root, filepath.FromSlash(repo.Manifest.Skills.Vendor))
	if err := os.MkdirAll(filepath.Dir(vendorRoot), 0o755); err != nil {
		return SkillSnapshot{}, err
	}
	stageParent, err := os.MkdirTemp(filepath.Dir(vendorRoot), ".ags-vendor-*")
	if err != nil {
		return SkillSnapshot{}, err
	}
	defer os.RemoveAll(stageParent)
	stageRoot := filepath.Join(stageParent, "snapshot")
	if err := os.MkdirAll(stageRoot, 0o755); err != nil {
		return SkillSnapshot{}, err
	}

	snapshot := SkillSnapshot{
		Version: snapshotVersion,
		Sources: copySourceLocks(repo.Lock.Sources),
		Include: append([]string{}, repo.Manifest.Skills.VendorInclude...),
		Skills:  map[string]SnapshotSkill{},
	}
	for _, unit := range selected {
		if previous := seen[unit.Name]; previous != "" {
			return SkillSnapshot{}, fmt.Errorf("Skill %q is provided by both %s and %s", unit.Name, previous, unit.Source)
		}
		seen[unit.Name] = unit.Source
		destination := filepath.Join(stageRoot, unit.Name)
		if err := copyTree(unit.Path, destination); err != nil {
			return SkillSnapshot{}, err
		}
		hash, err := TreeHash(destination)
		if err != nil {
			return SkillSnapshot{}, err
		}
		snapshot.Skills[unit.Name] = SnapshotSkill{Source: unit.Source, Hash: hash}
	}
	data, err := yaml.Marshal(snapshot)
	if err != nil {
		return SkillSnapshot{}, err
	}
	if err := os.WriteFile(filepath.Join(stageRoot, snapshotFilename), data, 0o644); err != nil {
		return SkillSnapshot{}, err
	}
	if err := replaceDirectory(stageRoot, vendorRoot); err != nil {
		return SkillSnapshot{}, err
	}
	return snapshot, nil
}

func copySourceLocks(source map[string]SourceLock) map[string]SourceLock {
	result := make(map[string]SourceLock, len(source))
	for name, lock := range source {
		result[name] = lock
	}
	return result
}

func replaceDirectory(source, target string) error {
	backup := target + ".ags-backup"
	if err := os.RemoveAll(backup); err != nil {
		return err
	}
	targetExists := false
	if _, err := os.Stat(target); err == nil {
		targetExists = true
		if err := os.Rename(target, backup); err != nil {
			return err
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	if err := os.Rename(source, target); err != nil {
		if targetExists {
			_ = os.Rename(backup, target)
		}
		return err
	}
	return os.RemoveAll(backup)
}

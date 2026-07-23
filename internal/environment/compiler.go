package environment

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"

	"github.com/shellus/ags/internal/agent"
	"github.com/shellus/ags/internal/command"
	"gopkg.in/yaml.v3"
)

type SkillUnit struct {
	Source string
	Name   string
	Path   string
}

type RenderedAgent struct {
	Name     agent.Name
	Stage    string
	Skills   []string
	Preserve []string
}

type BuildResult struct {
	Root        string
	Instruction []byte
	Agents      map[agent.Name]RenderedAgent
}

func (b BuildResult) Cleanup() error {
	return os.RemoveAll(b.Root)
}

type Compiler struct {
	CacheDir string
	Runner   command.Runner
}

func (c Compiler) runner() command.Runner {
	if c.Runner == nil {
		return command.OSRunner{}
	}
	return c.Runner
}

func (c Compiler) Build(repo Repository, profileName string, agents []agent.Name) (BuildResult, error) {
	buildRoot, err := os.MkdirTemp(filepath.Join(c.CacheDir, "builds"), "build-*")
	if err != nil {
		if mkdirErr := os.MkdirAll(filepath.Join(c.CacheDir, "builds"), 0o700); mkdirErr != nil {
			return BuildResult{}, fmt.Errorf("create build cache: %w", mkdirErr)
		}
		buildRoot, err = os.MkdirTemp(filepath.Join(c.CacheDir, "builds"), "build-*")
	}
	if err != nil {
		return BuildResult{}, fmt.Errorf("create build directory: %w", err)
	}
	result := BuildResult{Root: buildRoot, Agents: map[agent.Name]RenderedAgent{}}
	fail := func(err error) (BuildResult, error) {
		_ = result.Cleanup()
		return BuildResult{}, err
	}

	instructionPath := filepath.Join(repo.Root, filepath.FromSlash(repo.Manifest.Instructions.Global))
	result.Instruction, err = os.ReadFile(instructionPath)
	if err != nil {
		return fail(fmt.Errorf("read global instructions: %w", err))
	}

	resolved, err := c.resolveSources(repo, filepath.Join(buildRoot, "work"))
	if err != nil {
		return fail(err)
	}
	for _, name := range agents {
		profile, include, exclude, err := repo.Selection(profileName, name)
		if err != nil {
			return fail(err)
		}
		units, err := selectUnits(include, exclude, resolved)
		if err != nil {
			return fail(fmt.Errorf("select %s skills: %w", name, err))
		}
		stage := filepath.Join(buildRoot, "render", string(name))
		if err := os.MkdirAll(stage, 0o700); err != nil {
			return fail(err)
		}
		var skillNames []string
		for _, unit := range units {
			destination := filepath.Join(stage, unit.Name)
			if _, err := os.Stat(destination); err == nil {
				return fail(fmt.Errorf("duplicate rendered skill %q", unit.Name))
			}
			if err := copyTree(unit.Path, destination); err != nil {
				return fail(fmt.Errorf("render skill %s: %w", unit.Name, err))
			}
			skillNames = append(skillNames, unit.Name)
		}
		sort.Strings(skillNames)
		result.Agents[name] = RenderedAgent{Name: name, Stage: stage, Skills: skillNames, Preserve: append([]string{}, profile.Preserve...)}
	}
	return result, nil
}

func (c Compiler) resolveSources(repo Repository, workRoot string) (map[string][]SkillUnit, error) {
	if err := os.MkdirAll(workRoot, 0o700); err != nil {
		return nil, err
	}
	resolved := map[string][]SkillUnit{}
	checkoutByLock := map[string]string{}
	sourceNames := make([]string, 0, len(repo.Manifest.Sources))
	for name := range repo.Manifest.Sources {
		sourceNames = append(sourceNames, name)
	}
	sort.Strings(sourceNames)
	for _, name := range sourceNames {
		source := repo.Manifest.Sources[name]
		var sourcePath string
		switch source.Type {
		case "local":
			sourcePath = filepath.Join(repo.Root, filepath.FromSlash(source.Path))
		case "git":
			lockName := sourceLockName(name, source)
			if existing := checkoutByLock[lockName]; existing != "" {
				sourcePath = existing
				break
			}
			commit := repo.Lock.Sources[lockName].Commit
			checkout, err := c.checkoutGit(lockName, source.URL, commit)
			if err != nil {
				return nil, err
			}
			checkoutByLock[lockName] = checkout
			sourcePath = checkout
		}
		workPath := filepath.Join(workRoot, name)
		if err := copyTree(sourcePath, workPath); err != nil {
			return nil, fmt.Errorf("copy source %s: %w", name, err)
		}
		if err := runBuild(c.runner(), workPath, source); err != nil {
			return nil, fmt.Errorf("build source %s: %w", name, err)
		}
		if err := applyPatches(repo.Root, workPath, name, source.Patches); err != nil {
			return nil, err
		}
		units, err := discoverUnits(name, workPath, source.Discover)
		if err != nil {
			return nil, err
		}
		for _, unit := range units {
			if err := installNodeDependencies(c.runner(), unit); err != nil {
				return nil, err
			}
		}
		resolved[name] = units
	}
	return resolved, nil
}

func (c Compiler) checkoutGit(lockName, url, commit string) (string, error) {
	digest := fmt.Sprintf("%x", sha256.Sum256([]byte(lockName+"\x00"+url)))[:16]
	checkout := filepath.Join(c.CacheDir, "sources", digest)
	if _, err := os.Stat(filepath.Join(checkout, ".git")); os.IsNotExist(err) {
		if err := os.MkdirAll(filepath.Dir(checkout), 0o700); err != nil {
			return "", err
		}
		if err := os.RemoveAll(checkout); err != nil {
			return "", err
		}
		if _, err := c.runner().Run("", nil, "git", "clone", "--no-checkout", url, checkout); err != nil {
			return "", fmt.Errorf("clone source %s: %w", lockName, err)
		}
	}
	if _, err := c.runner().Run(checkout, nil, "git", "cat-file", "-e", commit+"^{commit}"); err != nil {
		if _, err := c.runner().Run(checkout, nil, "git", "fetch", "--force", "origin", commit); err != nil {
			return "", fmt.Errorf("fetch source %s commit %s: %w", lockName, commit, err)
		}
	}
	if _, err := c.runner().Run(checkout, nil, "git", "checkout", "--detach", "--force", commit); err != nil {
		return "", fmt.Errorf("checkout source %s commit %s: %w", lockName, commit, err)
	}
	if _, err := c.runner().Run(checkout, nil, "git", "clean", "-fdx"); err != nil {
		return "", fmt.Errorf("clean source %s: %w", lockName, err)
	}
	return checkout, nil
}

func runBuild(runner command.Runner, workPath string, source Source) error {
	if source.Build == nil {
		return nil
	}
	build := source.Build
	if build.Template != "" || build.Output != "" {
		if build.Template == "" || build.Output == "" {
			return fmt.Errorf("template and output must be configured together")
		}
		templatePath := filepath.Join(workPath, filepath.FromSlash(build.Template))
		outputPath := filepath.Join(workPath, filepath.FromSlash(build.Output))
		data, err := os.ReadFile(templatePath)
		if err != nil {
			return err
		}
		content := string(data)
		for key, value := range build.Replacements {
			content = strings.ReplaceAll(content, "{{"+key+"}}", value)
		}
		if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(outputPath, []byte(content), 0o644); err != nil {
			return err
		}
	}
	commands := append([]string{}, build.Commands["all"]...)
	commands = append(commands, build.Commands[runtime.GOOS]...)
	for _, value := range commands {
		var name string
		var args []string
		if runtime.GOOS == "windows" {
			name, args = "cmd", []string{"/C", value}
		} else {
			name, args = "sh", []string{"-c", value}
		}
		if _, err := runner.Run(workPath, nil, name, args...); err != nil {
			return err
		}
	}
	return nil
}

type patchDocument struct {
	Source  string      `yaml:"source"`
	Patches []patchItem `yaml:"patches"`
}

type patchItem struct {
	ID      string `yaml:"id"`
	File    string `yaml:"file"`
	Op      string `yaml:"op"`
	Start   string `yaml:"start"`
	End     string `yaml:"end"`
	Match   string `yaml:"match"`
	Replace string `yaml:"replace"`
}

func applyPatches(repoRoot, workPath, sourceName string, patchFiles []string) error {
	for _, relative := range patchFiles {
		path := filepath.Join(repoRoot, filepath.FromSlash(relative))
		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read patch %s: %w", relative, err)
		}
		var document patchDocument
		decoder := yaml.NewDecoder(strings.NewReader(string(data)))
		decoder.KnownFields(true)
		if err := decoder.Decode(&document); err != nil {
			return fmt.Errorf("parse patch %s: %w", relative, err)
		}
		if document.Source != sourceName {
			return fmt.Errorf("patch %s declares source %s, expected %s", relative, document.Source, sourceName)
		}
		for _, item := range document.Patches {
			filePath := filepath.Join(workPath, filepath.FromSlash(item.File))
			if _, err := os.Stat(filePath); os.IsNotExist(err) {
				continue
			}
			if err := applyPatchItem(filePath, item); err != nil {
				return fmt.Errorf("apply patch %s/%s: %w", relative, item.ID, err)
			}
		}
	}
	return nil
}

func applyPatchItem(path string, item patchItem) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	content := string(data)
	switch item.Op {
	case "delete_line":
		pattern, err := regexp.Compile(item.Match)
		if err != nil {
			return err
		}
		lines := strings.SplitAfter(content, "\n")
		kept := lines[:0]
		for _, line := range lines {
			if !pattern.MatchString(line) {
				kept = append(kept, line)
			}
		}
		content = strings.Join(kept, "")
	case "replace_line":
		pattern, err := regexp.Compile(item.Match)
		if err != nil {
			return err
		}
		lines := strings.SplitAfter(content, "\n")
		replaced := false
		for index, line := range lines {
			if pattern.MatchString(line) {
				ending := ""
				if strings.HasSuffix(line, "\n") {
					ending = "\n"
				}
				lines[index] = item.Replace + ending
				replaced = true
				break
			}
		}
		if !replaced {
			return fmt.Errorf("replace_line matched nothing")
		}
		content = strings.Join(lines, "")
	case "delete_block":
		start, err := regexp.Compile(item.Start)
		if err != nil {
			return err
		}
		end, err := regexp.Compile(item.End)
		if err != nil {
			return err
		}
		lines := strings.SplitAfter(content, "\n")
		startIndex := -1
		endIndex := len(lines)
		for index, line := range lines {
			if startIndex < 0 && start.MatchString(line) {
				startIndex = index
				continue
			}
			if startIndex >= 0 && end.MatchString(line) {
				endIndex = index + 1
				break
			}
		}
		if startIndex < 0 {
			return nil
		}
		content = strings.Join(append(lines[:startIndex], lines[endIndex:]...), "")
	default:
		return fmt.Errorf("unsupported patch operation %q", item.Op)
	}
	return os.WriteFile(path, []byte(content), 0o644)
}

func discoverUnits(sourceName, workPath string, discover Discover) ([]SkillUnit, error) {
	root := filepath.Join(workPath, filepath.FromSlash(discover.Path))
	switch discover.Mode {
	case "single":
		return singleUnit(sourceName, root)
	case "flat", "collection":
		entries, err := os.ReadDir(root)
		if err != nil {
			return nil, fmt.Errorf("discover source %s: %w", sourceName, err)
		}
		var units []SkillUnit
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			path := filepath.Join(root, entry.Name())
			if _, err := os.Stat(filepath.Join(path, "SKILL.md")); err != nil {
				continue
			}
			unit, err := makeUnit(sourceName, path)
			if err != nil {
				return nil, err
			}
			units = append(units, unit)
		}
		sort.Slice(units, func(i, j int) bool { return units[i].Name < units[j].Name })
		return units, nil
	default:
		return nil, fmt.Errorf("unsupported discover mode %q", discover.Mode)
	}
}

func singleUnit(sourceName, root string) ([]SkillUnit, error) {
	if _, err := os.Stat(filepath.Join(root, "SKILL.md")); err != nil {
		return nil, fmt.Errorf("single source %s lacks SKILL.md: %w", sourceName, err)
	}
	unit, err := makeUnit(sourceName, root)
	if err != nil {
		return nil, err
	}
	return []SkillUnit{unit}, nil
}

func makeUnit(sourceName, path string) (SkillUnit, error) {
	name, err := frontmatterName(filepath.Join(path, "SKILL.md"))
	if err != nil {
		return SkillUnit{}, err
	}
	if name == "" {
		name = filepath.Base(path)
	}
	if !safeName(name) {
		return SkillUnit{}, fmt.Errorf("unsafe skill name %q", name)
	}
	return SkillUnit{Source: sourceName, Name: name, Path: path}, nil
}

func frontmatterName(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	inFrontmatter := false
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "---" {
			if inFrontmatter {
				break
			}
			inFrontmatter = true
			continue
		}
		if inFrontmatter && strings.HasPrefix(line, "name:") {
			return strings.Trim(strings.TrimSpace(strings.TrimPrefix(line, "name:")), `"'`), nil
		}
	}
	return "", scanner.Err()
}

func installNodeDependencies(runner command.Runner, unit SkillUnit) error {
	packageJSON := filepath.Join(unit.Path, "package.json")
	if _, err := os.Stat(packageJSON); os.IsNotExist(err) {
		return nil
	}
	if _, err := os.Stat(filepath.Join(unit.Path, "package-lock.json")); err != nil {
		return fmt.Errorf("Node skill %s:%s requires package-lock.json", unit.Source, unit.Name)
	}
	if _, err := runner.Run(unit.Path, []string{"PLAYWRIGHT_SKIP_BROWSER_DOWNLOAD=1"}, "npm", "ci", "--omit=dev", "--no-audit", "--no-fund"); err != nil {
		return fmt.Errorf("install Node dependencies for %s:%s: %w", unit.Source, unit.Name, err)
	}
	return nil
}

func selectUnits(include, exclude []string, sources map[string][]SkillUnit) ([]SkillUnit, error) {
	excluded := map[string]bool{}
	for _, item := range exclude {
		excluded[item] = true
	}
	var all []SkillUnit
	for _, units := range sources {
		all = append(all, units...)
	}
	sort.Slice(all, func(i, j int) bool {
		if all[i].Source == all[j].Source {
			return all[i].Name < all[j].Name
		}
		return all[i].Source < all[j].Source
	})
	seenUnit := map[string]bool{}
	seenName := map[string]string{}
	var selected []SkillUnit
	for _, pattern := range include {
		matched := false
		for _, unit := range all {
			if !unitMatches(pattern, unit, sources) {
				continue
			}
			matched = true
			if excluded[unit.Name] || excluded[unit.Source+":"+unit.Name] {
				continue
			}
			key := unit.Source + ":" + unit.Name
			if seenUnit[key] {
				continue
			}
			if previous := seenName[unit.Name]; previous != "" && previous != unit.Source {
				return nil, fmt.Errorf("skill name %q is provided by both %s and %s", unit.Name, previous, unit.Source)
			}
			seenUnit[key] = true
			seenName[unit.Name] = unit.Source
			selected = append(selected, unit)
		}
		if !matched {
			return nil, fmt.Errorf("include pattern %q matched no skills", pattern)
		}
	}
	return selected, nil
}

func unitMatches(pattern string, unit SkillUnit, sources map[string][]SkillUnit) bool {
	if pattern == unit.Source && len(sources[unit.Source]) == 1 {
		return true
	}
	if pattern == unit.Name || pattern == unit.Source+":"+unit.Name {
		return true
	}
	if strings.HasPrefix(pattern, unit.Source+":") {
		matched, _ := filepath.Match(strings.TrimPrefix(pattern, unit.Source+":"), unit.Name)
		return matched
	}
	return false
}

func copyTree(source, target string) error {
	info, err := os.Stat(source)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("source is not a directory: %s", source)
	}
	return filepath.WalkDir(source, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relative, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		if relative == ".git" || strings.HasPrefix(relative, ".git"+string(filepath.Separator)) {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		destination := filepath.Join(target, relative)
		entryInfo, err := entry.Info()
		if err != nil {
			return err
		}
		if entryInfo.Mode()&os.ModeSymlink != 0 {
			linkTarget, err := os.Readlink(path)
			if err != nil {
				return err
			}
			if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
				return err
			}
			return os.Symlink(linkTarget, destination)
		}
		if entry.IsDir() {
			return os.MkdirAll(destination, entryInfo.Mode().Perm())
		}
		if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
			return err
		}
		src, err := os.Open(path)
		if err != nil {
			return err
		}
		dst, err := os.OpenFile(destination, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, entryInfo.Mode().Perm())
		if err != nil {
			src.Close()
			return err
		}
		_, copyErr := io.Copy(dst, src)
		sourceCloseErr := src.Close()
		closeErr := dst.Close()
		if copyErr != nil {
			return copyErr
		}
		if sourceCloseErr != nil {
			return sourceCloseErr
		}
		return closeErr
	})
}

func safeName(name string) bool {
	return name != "" && name != "." && name != ".." && !strings.ContainsAny(name, `/\\`)
}

func TreeHash(root string) (string, error) {
	hash := sha256.New()
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if _, err := io.WriteString(hash, filepath.ToSlash(relative)+"\x00"); err != nil {
			return err
		}
		file, err := os.Open(path)
		if err != nil {
			return err
		}
		_, copyErr := io.Copy(hash, file)
		closeErr := file.Close()
		if copyErr != nil {
			return copyErr
		}
		return closeErr
	})
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

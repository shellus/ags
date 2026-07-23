package environment

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/shellus/ags/internal/agent"
	"github.com/shellus/ags/internal/configfile"
	"github.com/shellus/ags/internal/localconfig"
)

type SkillChange struct {
	Name string
	Kind string
}

type AgentPlan struct {
	Name                   agent.Name
	Package                agent.Package
	InstalledVersion       string
	DesiredVersion         string
	InstructionChanged     bool
	InstructionDestination string
	SkillsDestination      string
	DesiredSkills          map[string]string
	ManagedBefore          map[string]string
	TakeoverBefore         map[string]string
	SkillChanges           []SkillChange
	Stage                  string
}

type Plan struct {
	Source     string
	Branch     string
	Commit     string
	Profile    string
	Repository Repository
	Build      BuildResult
	Agents     []AgentPlan
	Config     localconfig.Config
}

func (p *Plan) Cleanup() error {
	return p.Build.Cleanup()
}

func (p Plan) HasChanges() bool {
	for _, item := range p.Agents {
		if item.InstalledVersion != item.DesiredVersion || item.InstructionChanged || len(item.SkillChanges) > 0 {
			return true
		}
	}
	return false
}

func preparePlan(paths configfile.Paths, config localconfig.Config, source, branch, commit, profile string, repo Repository, build BuildResult, installedVersions map[agent.Name]string) (Plan, error) {
	state, err := LoadState(filepath.Join(paths.StateDir, "environment.json"))
	if err != nil {
		return Plan{}, err
	}
	plan := Plan{Source: source, Branch: branch, Commit: commit, Profile: profile, Repository: repo, Build: build, Config: config}
	desiredInstructionHash := bytesHash(build.Instruction)
	for name, rendered := range build.Agents {
		pkg, err := repo.AgentPackage(name)
		if err != nil {
			return Plan{}, err
		}
		guidance, skillsDir, err := agentDestinations(paths, name)
		if err != nil {
			return Plan{}, err
		}
		marker, err := readMarker(skillsDir)
		if err != nil {
			return Plan{}, err
		}
		managedBefore := marker.Skills
		if len(managedBefore) == 0 {
			if previous, ok := state.Agents[name]; ok && previous.Skills != nil {
				managedBefore = previous.Skills
			}
		}
		desiredSkills := map[string]string{}
		for _, skillName := range rendered.Skills {
			hash, err := TreeHash(filepath.Join(rendered.Stage, skillName))
			if err != nil {
				return Plan{}, err
			}
			desiredSkills[skillName] = hash
		}
		item := AgentPlan{
			Name:                   name,
			Package:                pkg,
			InstalledVersion:       installedVersions[name],
			DesiredVersion:         pkg.Version,
			InstructionDestination: guidance,
			SkillsDestination:      skillsDir,
			DesiredSkills:          desiredSkills,
			ManagedBefore:          managedBefore,
			TakeoverBefore:         map[string]string{},
			Stage:                  rendered.Stage,
		}
		currentInstructionHash, err := fileHash(guidance)
		if err != nil {
			return Plan{}, err
		}
		item.InstructionChanged = currentInstructionHash != desiredInstructionHash
		for skillName, desiredHash := range desiredSkills {
			path := filepath.Join(skillsDir, skillName)
			currentHash, hashErr := TreeHash(path)
			if errors.Is(hashErr, os.ErrNotExist) {
				item.SkillChanges = append(item.SkillChanges, SkillChange{Name: skillName, Kind: "add"})
				continue
			}
			if hashErr != nil {
				return Plan{}, hashErr
			}
			if _, managed := managedBefore[skillName]; !managed {
				item.TakeoverBefore[skillName] = currentHash
				item.SkillChanges = append(item.SkillChanges, SkillChange{Name: skillName, Kind: "takeover"})
				continue
			}
			if currentHash != desiredHash {
				item.SkillChanges = append(item.SkillChanges, SkillChange{Name: skillName, Kind: "update"})
			}
		}
		for skillName := range managedBefore {
			if _, ok := desiredSkills[skillName]; !ok {
				item.SkillChanges = append(item.SkillChanges, SkillChange{Name: skillName, Kind: "remove"})
			}
		}
		sort.Slice(item.SkillChanges, func(i, j int) bool {
			if item.SkillChanges[i].Kind == item.SkillChanges[j].Kind {
				return item.SkillChanges[i].Name < item.SkillChanges[j].Name
			}
			return item.SkillChanges[i].Kind < item.SkillChanges[j].Kind
		})
		plan.Agents = append(plan.Agents, item)
	}
	sort.Slice(plan.Agents, func(i, j int) bool { return plan.Agents[i].Name < plan.Agents[j].Name })
	return plan, nil
}

func agentDestinations(paths configfile.Paths, name agent.Name) (string, string, error) {
	switch name {
	case agent.Codex:
		return paths.CodexGuidance, paths.CodexSkills, nil
	case agent.Claude:
		return paths.ClaudeGuidance, paths.ClaudeSkills, nil
	default:
		return "", "", fmt.Errorf("unsupported agent %s", name)
	}
}

func bytesHash(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func fileHash(path string) (string, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return bytesHash(data), nil
}

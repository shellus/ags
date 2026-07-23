package agent

import (
	"fmt"
	"strings"
)

type Name string

const (
	Codex  Name = "codex"
	Claude Name = "claude"
	All    Name = "all"
)

func Parse(value string, allowAll bool) (Name, error) {
	name := Name(strings.ToLower(strings.TrimSpace(value)))
	switch name {
	case Codex, Claude:
		return name, nil
	case All:
		if allowAll {
			return name, nil
		}
	}
	if allowAll {
		return "", fmt.Errorf("unknown agent %q; expected codex, claude, or all", value)
	}
	return "", fmt.Errorf("unknown agent %q; expected codex or claude", value)
}

func Expand(names []Name) ([]Name, error) {
	seen := map[Name]bool{}
	var result []Name
	for _, name := range names {
		parsed, err := Parse(string(name), true)
		if err != nil {
			return nil, err
		}
		if parsed == All {
			for _, item := range []Name{Codex, Claude} {
				if !seen[item] {
					seen[item] = true
					result = append(result, item)
				}
			}
			continue
		}
		if !seen[parsed] {
			seen[parsed] = true
			result = append(result, parsed)
		}
	}
	return result, nil
}

func AllNames() []Name {
	return []Name{Codex, Claude}
}

func DefaultNPMName(name Name) (string, error) {
	switch name {
	case Codex:
		return "@openai/codex", nil
	case Claude:
		return "@anthropic-ai/claude-code", nil
	default:
		return "", fmt.Errorf("unsupported agent %s", name)
	}
}

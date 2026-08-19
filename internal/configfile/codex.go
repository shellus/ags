package configfile

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

const (
	codexDisabledSkillsBegin = "# BEGIN AGS MANAGED CODEX DISABLED SKILLS"
	codexDisabledSkillsEnd   = "# END AGS MANAGED CODEX DISABLED SKILLS"
)

// UpdateCodexDisabledSkills replaces the AGS-owned disabled Skills block and
// preserves all other Codex configuration text byte-for-byte.
func UpdateCodexDisabledSkills(config string, disabledSkills []string) (string, error) {
	lines := splitConfigLines(config)
	start, end := -1, -1
	for index, line := range lines {
		plain := strings.TrimSpace(strings.TrimRight(line, "\r\n"))
		switch plain {
		case codexDisabledSkillsBegin:
			if start >= 0 {
				return "", fmt.Errorf("Codex config contains more than one AGS disabled Skills block")
			}
			start = index
		case codexDisabledSkillsEnd:
			if start < 0 || end >= 0 {
				return "", fmt.Errorf("Codex config contains an unmatched AGS disabled Skills block marker")
			}
			end = index
		}
	}
	if (start >= 0) != (end >= 0) || (start >= 0 && end < start) {
		return "", fmt.Errorf("Codex config contains an incomplete AGS disabled Skills block")
	}

	if start >= 0 {
		removeStart := start
		if removeStart > 0 && strings.TrimSpace(strings.TrimRight(lines[removeStart-1], "\r\n")) == "" {
			removeStart--
		}
		lines = append(lines[:removeStart], lines[end+1:]...)
	}
	base := strings.Join(lines, "")
	if len(disabledSkills) == 0 {
		return base, nil
	}

	names := append([]string{}, disabledSkills...)
	sort.Strings(names)
	newline := "\n"
	if strings.Contains(config, "\r\n") {
		newline = "\r\n"
	}
	var output strings.Builder
	output.WriteString(base)
	if base != "" {
		if !strings.HasSuffix(base, "\n") {
			output.WriteString(newline)
		}
		output.WriteString(newline)
	}
	output.WriteString(codexDisabledSkillsBegin)
	output.WriteString(newline)
	for _, name := range names {
		output.WriteString("[[skills.config]]")
		output.WriteString(newline)
		output.WriteString("name = ")
		output.WriteString(strconv.Quote(name))
		output.WriteString(newline)
		output.WriteString("enabled = false")
		output.WriteString(newline)
	}
	output.WriteString(codexDisabledSkillsEnd)
	output.WriteString(newline)
	return output.String(), nil
}

func splitConfigLines(value string) []string {
	if value == "" {
		return nil
	}
	lines := strings.SplitAfter(value, "\n")
	if lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

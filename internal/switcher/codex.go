package switcher

import (
	"errors"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/shellus/ags/internal/configfile"
	"github.com/shellus/ags/internal/registry"
	"github.com/shellus/ags/internal/transaction"
)

var (
	tomlSectionPattern = regexp.MustCompile(`^\s*\[([^\]]+)\]\s*(?:#.*)?$`)
	tomlBaseURLPattern = regexp.MustCompile(`^(\s*)base_url\s*=`)
	tomlStringPattern  = regexp.MustCompile(`^\s*base_url\s*=\s*("(?:\\.|[^"])*"|'[^']*')`)
)

func prepareCodex(paths configfile.Paths, provider registry.CodexConfig) ([]transaction.Change, error) {
	auth, err := os.ReadFile(paths.CodexAuth)
	if err != nil {
		return nil, fmt.Errorf("read Codex auth file %s: %w", paths.CodexAuth, err)
	}
	updatedAuth, err := setJSONString(auth, "OPENAI_API_KEY", provider.APIKey)
	if err != nil {
		return nil, fmt.Errorf("update Codex auth file %s: %w", paths.CodexAuth, err)
	}

	config, err := os.ReadFile(paths.CodexConfig)
	if err != nil {
		return nil, fmt.Errorf("read Codex config file %s: %w", paths.CodexConfig, err)
	}
	updatedConfig, err := replaceCodexBaseURL(string(config), provider.BaseURL)
	if err != nil {
		return nil, fmt.Errorf("update Codex config file %s: %w", paths.CodexConfig, err)
	}

	return []transaction.Change{
		{Path: paths.CodexAuth, Content: updatedAuth},
		{Path: paths.CodexConfig, Content: []byte(updatedConfig)},
	}, nil
}

func readCodexCurrent(paths configfile.Paths) (string, string, error) {
	auth, err := os.ReadFile(paths.CodexAuth)
	if errors.Is(err, os.ErrNotExist) {
		return "", "", nil
	}
	if err != nil {
		return "", "", fmt.Errorf("read Codex auth file %s: %w", paths.CodexAuth, err)
	}
	apiKey, err := readJSONString(auth, "OPENAI_API_KEY")
	if err != nil {
		return "", "", fmt.Errorf("parse Codex auth file %s: %w", paths.CodexAuth, err)
	}

	config, err := os.ReadFile(paths.CodexConfig)
	if errors.Is(err, os.ErrNotExist) {
		return "", "", nil
	}
	if err != nil {
		return "", "", fmt.Errorf("read Codex config file %s: %w", paths.CodexConfig, err)
	}
	baseURL, err := readCodexBaseURL(string(config))
	if err != nil {
		return "", "", fmt.Errorf("parse Codex config file %s: %w", paths.CodexConfig, err)
	}
	return apiKey, baseURL, nil
}

func replaceCodexBaseURL(config, baseURL string) (string, error) {
	lines := splitLines(config)
	newline := preferredNewline(config)
	output := make([]string, 0, len(lines)+1)
	inCustom := false
	sawCustom := false
	replaced := false
	inserted := false

	for _, line := range lines {
		plain, ending := splitLineEnding(line)
		if match := tomlSectionPattern.FindStringSubmatch(plain); match != nil {
			if inCustom && !replaced {
				output = append(output, "base_url = "+strconv.Quote(baseURL)+newline)
				inserted = true
			}
			inCustom = strings.TrimSpace(match[1]) == "model_providers.custom"
			sawCustom = sawCustom || inCustom
		}

		if inCustom {
			if match := tomlBaseURLPattern.FindStringSubmatch(plain); match != nil {
				output = append(output, match[1]+"base_url = "+strconv.Quote(baseURL)+ending)
				replaced = true
				continue
			}
		}
		output = append(output, line)
	}

	if !sawCustom {
		return "", fmt.Errorf("missing [model_providers.custom] section")
	}
	if inCustom && !replaced && !inserted {
		if len(output) > 0 && !hasLineEnding(output[len(output)-1]) {
			output[len(output)-1] += newline
		}
		output = append(output, "base_url = "+strconv.Quote(baseURL)+newline)
	}
	return strings.Join(output, ""), nil
}

func readCodexBaseURL(config string) (string, error) {
	inCustom := false
	for _, line := range splitLines(config) {
		plain, _ := splitLineEnding(line)
		if match := tomlSectionPattern.FindStringSubmatch(plain); match != nil {
			inCustom = strings.TrimSpace(match[1]) == "model_providers.custom"
			continue
		}
		if !inCustom {
			continue
		}
		match := tomlStringPattern.FindStringSubmatch(plain)
		if match == nil {
			continue
		}
		value := match[1]
		if strings.HasPrefix(value, "'") {
			return strings.Trim(value, "'"), nil
		}
		unquoted, err := strconv.Unquote(value)
		if err != nil {
			return "", err
		}
		return unquoted, nil
	}
	return "", nil
}

func splitLines(value string) []string {
	if value == "" {
		return nil
	}
	lines := strings.SplitAfter(value, "\n")
	if lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

func preferredNewline(value string) string {
	if strings.Contains(value, "\r\n") {
		return "\r\n"
	}
	return "\n"
}

func splitLineEnding(line string) (string, string) {
	if strings.HasSuffix(line, "\r\n") {
		return strings.TrimSuffix(line, "\r\n"), "\r\n"
	}
	if strings.HasSuffix(line, "\n") {
		return strings.TrimSuffix(line, "\n"), "\n"
	}
	return line, ""
}

func hasLineEnding(line string) bool {
	return strings.HasSuffix(line, "\n")
}

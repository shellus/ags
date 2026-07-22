package switcher

import (
	"errors"
	"fmt"
	"os"

	"github.com/shellus/ags/internal/configfile"
	"github.com/shellus/ags/internal/registry"
	"github.com/shellus/ags/internal/transaction"
)

func prepareClaude(paths configfile.Paths, provider registry.ClaudeProvider) ([]transaction.Change, error) {
	settings, err := os.ReadFile(paths.ClaudeSettings)
	if errors.Is(err, os.ErrNotExist) {
		settings = []byte("{}\n")
	} else if err != nil {
		return nil, fmt.Errorf("read Claude settings file %s: %w", paths.ClaudeSettings, err)
	}

	updated, err := setJSONString(settings, "env.ANTHROPIC_AUTH_TOKEN", provider.AuthToken)
	if err != nil {
		return nil, fmt.Errorf("update Claude settings file %s: %w", paths.ClaudeSettings, err)
	}
	updated, err = setJSONString(updated, "env.ANTHROPIC_BASE_URL", provider.BaseURL)
	if err != nil {
		return nil, fmt.Errorf("update Claude settings file %s: %w", paths.ClaudeSettings, err)
	}
	if provider.Model != "" {
		updated, err = setJSONString(updated, "model", provider.Model)
		if err != nil {
			return nil, fmt.Errorf("update Claude settings file %s: %w", paths.ClaudeSettings, err)
		}
	}

	return []transaction.Change{{Path: paths.ClaudeSettings, Content: updated}}, nil
}

func readClaudeCurrent(paths configfile.Paths) (string, string, string, error) {
	settings, err := os.ReadFile(paths.ClaudeSettings)
	if errors.Is(err, os.ErrNotExist) {
		return "", "", "", nil
	}
	if err != nil {
		return "", "", "", fmt.Errorf("read Claude settings file %s: %w", paths.ClaudeSettings, err)
	}
	authToken, err := readJSONString(settings, "env", "ANTHROPIC_AUTH_TOKEN")
	if err != nil {
		return "", "", "", fmt.Errorf("parse Claude settings file %s: %w", paths.ClaudeSettings, err)
	}
	baseURL, err := readJSONString(settings, "env", "ANTHROPIC_BASE_URL")
	if err != nil {
		return "", "", "", fmt.Errorf("parse Claude settings file %s: %w", paths.ClaudeSettings, err)
	}
	model, err := readJSONString(settings, "model")
	if err != nil {
		return "", "", "", fmt.Errorf("parse Claude settings file %s: %w", paths.ClaudeSettings, err)
	}
	return authToken, baseURL, model, nil
}

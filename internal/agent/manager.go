package agent

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/shellus/ags/internal/command"
)

type Package struct {
	Name    Name
	NPMName string
	Version string
}

type Manager struct {
	Runner command.Runner
}

func (m Manager) runner() command.Runner {
	if m.Runner == nil {
		return command.OSRunner{}
	}
	return m.Runner
}

func (m Manager) InstalledVersion(pkg Package) (string, error) {
	output, err := m.runner().Run("", nil, "npm", "root", "-g")
	if err != nil {
		return "", err
	}
	root := strings.TrimSpace(string(output))
	if root == "" {
		return "", fmt.Errorf("npm root -g returned an empty path")
	}
	packagePath := filepath.Join(append([]string{root}, strings.Split(pkg.NPMName, "/")...)...)
	data, err := os.ReadFile(filepath.Join(packagePath, "package.json"))
	if errors.Is(err, os.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("read installed package %s: %w", pkg.NPMName, err)
	}
	var metadata struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal(data, &metadata); err != nil {
		return "", fmt.Errorf("parse installed package %s: %w", pkg.NPMName, err)
	}
	return strings.TrimSpace(metadata.Version), nil
}

func (m Manager) Install(pkg Package) error {
	if strings.TrimSpace(pkg.NPMName) == "" || strings.TrimSpace(pkg.Version) == "" {
		return fmt.Errorf("agent %s package and version must not be empty", pkg.Name)
	}
	_, err := m.runner().Run("", nil, "npm", "install", "-g", pkg.NPMName+"@"+pkg.Version)
	if err != nil {
		return fmt.Errorf("install %s %s: %w", pkg.Name, pkg.Version, err)
	}
	return nil
}

func (m Manager) Uninstall(pkg Package) error {
	if strings.TrimSpace(pkg.NPMName) == "" {
		return fmt.Errorf("agent %s package must not be empty", pkg.Name)
	}
	_, err := m.runner().Run("", nil, "npm", "uninstall", "-g", pkg.NPMName)
	if err != nil {
		return fmt.Errorf("uninstall %s: %w", pkg.Name, err)
	}
	return nil
}

func (m Manager) LatestVersion(npmPackage string) (string, error) {
	output, err := m.runner().Run("", nil, "npm", "view", npmPackage, "version")
	if err != nil {
		return "", err
	}
	version := strings.TrimSpace(string(output))
	if version == "" {
		return "", fmt.Errorf("npm returned an empty version for %s", npmPackage)
	}
	return version, nil
}

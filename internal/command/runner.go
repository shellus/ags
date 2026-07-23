package command

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

type Runner interface {
	Run(dir string, extraEnv []string, name string, args ...string) ([]byte, error)
	LookPath(name string) (string, error)
}

type OSRunner struct{}

func (OSRunner) Run(dir string, extraEnv []string, name string, args ...string) ([]byte, error) {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	if len(extraEnv) > 0 {
		cmd.Env = append(os.Environ(), extraEnv...)
	}
	output, err := cmd.CombinedOutput()
	if err != nil {
		message := strings.TrimSpace(string(output))
		if message == "" {
			return output, fmt.Errorf("run %s: %w", name, err)
		}
		return output, fmt.Errorf("run %s: %w: %s", name, err, message)
	}
	return output, nil
}

func (OSRunner) LookPath(name string) (string, error) {
	return exec.LookPath(name)
}

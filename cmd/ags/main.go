package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/shellus/ags/internal/app"
	"github.com/shellus/ags/internal/configfile"
	"github.com/shellus/ags/internal/interactive"
)

func main() {
	paths, err := configfile.ResolvePaths()
	if err != nil {
		fmt.Fprintf(os.Stderr, "ags: %v\n", err)
		os.Exit(1)
	}

	runner := app.Runner{
		Paths:       paths,
		Out:         os.Stdout,
		UI:          interactive.Selector{},
		Interactive: isInteractiveTerminal(),
	}
	if err := runner.Run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "ags: %v\n", err)

		var usageErr *app.UsageError
		if errors.As(err, &usageErr) {
			os.Exit(2)
		}
		os.Exit(1)
	}
}

func isInteractiveTerminal() bool {
	info, err := os.Stdin.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}

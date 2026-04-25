package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"

	"github.com/sang-bin/vscode-color-workspace/internal/interactive"
	"github.com/sang-bin/vscode-color-workspace/internal/runner"
)

func interactiveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "interactive [target-dir]",
		Short: "Walk through options interactively (huh form).",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runInteractive(args)
		},
	}
}

func runInteractive(args []string) error {
	target := "."
	if len(args) == 1 {
		target = args[0]
	}
	abs, err := filepath.Abs(target)
	if err != nil {
		return err
	}
	choices, err := interactive.Run(abs)
	if err != nil {
		return err
	}

	opts := interactive.ApplyToOptions(*choices, choices.TargetDir)

	for attempt := 0; attempt < 2; attempt++ {
		res, err := runner.New(nil).Run(opts)
		if err == nil {
			fmt.Printf("wrote %s\n", res.WorkspaceFile)
			fmt.Printf("color: %s\n", res.ColorHex)
			for _, w := range res.Warnings {
				fmt.Fprintln(os.Stderr, "warning: "+w)
			}
			return nil
		}
		var ge *runner.GuardError
		if !errors.As(err, &ge) {
			return err
		}
		if attempt > 0 {
			return err
		}
		ok, cerr := confirmGuard(ge)
		if cerr != nil {
			return cerr
		}
		if !ok {
			return fmt.Errorf("aborted (guard %d)", ge.Guard)
		}
		opts.Force = true
	}
	return nil
}

func confirmGuard(ge *runner.GuardError) (bool, error) {
	title := fmt.Sprintf("Guard %d triggered", ge.Guard)
	desc := ge.Error() + "\n\nKeys:\n  " + strings.Join(ge.Keys, "\n  ")
	var proceed bool
	err := huh.NewConfirm().
		Title(title).
		Description(desc).
		Affirmative("Override").
		Negative("Abort").
		Value(&proceed).
		Run()
	return proceed, err
}

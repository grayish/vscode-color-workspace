package main

import (
	"errors"
	"fmt"
	"path/filepath"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"

	"github.com/sang-bin/vscode-color-workspace/internal/interactive"
	"github.com/sang-bin/vscode-color-workspace/internal/runner"
	"github.com/sang-bin/vscode-color-workspace/internal/tui"
	"github.com/sang-bin/vscode-color-workspace/internal/workspace"
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

// detectPreconfigured returns the workspace file path and existing peacock
// keys when target/<...>.code-workspace already has peacock keys; otherwise
// returns ("", nil, nil). A read error (other than "not exist") is returned
// to the caller.
func detectPreconfigured(target string) (string, []string, error) {
	abs, err := filepath.Abs(target)
	if err != nil {
		return "", nil, err
	}
	parent := filepath.Dir(abs)
	folderName := filepath.Base(abs)
	wsPath := filepath.Join(parent, folderName+".code-workspace")
	ws, err := workspace.Read(wsPath)
	if err != nil {
		return "", nil, err
	}
	if ws == nil {
		return "", nil, nil
	}
	keys := workspace.ExistingPeacockKeys(ws)
	if len(keys) == 0 {
		return "", nil, nil
	}
	return wsPath, keys, nil
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
			renderSuccess(tui.NewStdout(), res, "")
			renderWarnings(tui.NewStderr(), res.Warnings)
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
	desc := guardDescription(ge)
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

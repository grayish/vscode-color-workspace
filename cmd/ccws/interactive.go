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

	// Phase A: when an existing peacock workspace is detected, offer a 3-option
	// pre-check before the form. The huh.Select itself is not unit-tested
	// (interactive UI); the data layer is covered by runner.CheckPreconfigured
	// tests. Phase A is skipped entirely when len(keys) == 0.
	wsPath, keys, err := runner.CheckPreconfigured(abs)
	if err != nil {
		return err
	}
	forcePreselected := false
	if len(keys) > 0 {
		var choice string
		desc := fmt.Sprintf("%s\n%d peacock keys present", tui.ShortenPath(wsPath), len(keys))
		if err := huh.NewSelect[string]().
			Title("Workspace already configured").
			Description(desc).
			Options(
				huh.NewOption("Open existing workspace", "open"),
				huh.NewOption("Overwrite (start fresh)", "overwrite"),
				huh.NewOption("Cancel", "cancel"),
			).
			Value(&choice).
			Run(); err != nil {
			return err
		}
		switch choice {
		case "open":
			opts := runner.Defaults()
			opts.TargetDir = abs
			res, err := runner.New(nil).Run(opts)
			if err != nil {
				return err
			}
			renderPreconfigured(tui.NewStderr(), res)
			renderWarnings(tui.NewStderr(), res.Warnings)
			return nil
		case "cancel":
			return nil
		case "overwrite":
			forcePreselected = true
			// fall through to Phase B
		}
	}

	// Phase B: regular form flow.
	choices, err := interactive.Run(abs)
	if err != nil {
		return err
	}
	opts := interactive.ApplyToOptions(*choices, choices.TargetDir)
	if forcePreselected {
		opts.Force = true
	}

	for attempt := 0; attempt < 2; attempt++ {
		res, err := runner.New(nil).Run(opts)
		if err == nil {
			renderInteractiveResult(res)
			return nil
		}
		var ge *runner.GuardError
		if !errors.As(err, &ge) {
			if !errors.Is(err, runner.ErrPartialPropagation) {
				return err
			}
			// ErrPartialPropagation: res is populated; render then propagate.
			renderInteractiveResult(res)
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

func renderInteractiveResult(res *runner.Result) {
	if res.Preconfigured {
		renderPreconfigured(tui.NewStderr(), res)
	} else {
		renderSuccess(tui.NewStdout(), res, "")
	}
	renderWarnings(tui.NewStderr(), res.Warnings)
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

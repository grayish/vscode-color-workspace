package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/sang-bin/vscode-color-workspace/internal/runner"
)

// errToExit maps error types to exit codes (§11 of spec).
func errToExit(err error) int {
	if err == nil {
		return 0
	}
	var ge *runner.GuardError
	if errors.As(err, &ge) {
		return 2
	}
	if errors.Is(err, os.ErrPermission) {
		return 3
	}
	return 1
}

func rootCmd() *cobra.Command {
	var (
		flagColor  string
		flagNoOpen bool
		flagForce  bool
	)

	cmd := &cobra.Command{
		Use:   "ccws [target-dir]",
		Short: "Create a .code-workspace file with Peacock-style colors and open it.",
		Long: `ccws generates a <parent>/<folder>.code-workspace file containing a
Peacock-equivalent color palette, migrates existing peacock settings from
<target>/.vscode/settings.json, and opens the workspace in VSCode.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			target := "."
			if len(args) == 1 {
				target = args[0]
			}
			opts := runner.Defaults()
			opts.TargetDir = target
			opts.ColorInput = flagColor
			opts.NoOpen = flagNoOpen
			opts.Force = flagForce
			res, err := runner.New(nil).Run(opts)
			if err != nil {
				return err
			}
			fmt.Printf("wrote %s\n", res.WorkspaceFile)
			fmt.Printf("color: %s (%s)\n", res.ColorHex, sourceLabel(res.ColorSource))
			for _, w := range res.Warnings {
				fmt.Fprintln(os.Stderr, "warning: "+w)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&flagColor, "color", "", "Color: #RRGGBB, #RGB, CSS name, or 'random'")
	cmd.Flags().BoolVar(&flagNoOpen, "no-open", false, "Do not open with the code CLI after creating")
	cmd.Flags().BoolVar(&flagForce, "force", false, "Bypass safety guards")

	cmd.AddCommand(interactiveCmd())
	return cmd
}

func sourceLabel(s runner.ColorSource) string {
	switch s {
	case runner.SourceFlag:
		return "from --color"
	case runner.SourceSettings:
		return "inherited from .vscode/settings.json"
	case runner.SourceRandom:
		return "random"
	default:
		return "?"
	}
}

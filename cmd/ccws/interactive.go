package main

import "github.com/spf13/cobra"

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
	return nil // implemented in Task 28
}

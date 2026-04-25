package main

import (
	"os"

	"github.com/sang-bin/vscode-color-workspace/internal/tui"
)

func main() {
	cmd := rootCmd()
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true
	if err := cmd.Execute(); err != nil {
		renderError(tui.NewStderr(), err)
		os.Exit(errToExit(err))
	}
}

package main

import (
	"os"

	"github.com/sang-bin/vscode-color-workspace/internal/tui"
)

// version is overridden at build time via -ldflags "-X main.version=...".
var version = "dev"

func main() {
	cmd := rootCmd()
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true
	if err := cmd.Execute(); err != nil {
		renderError(tui.NewStderr(), err)
		os.Exit(errToExit(err))
	}
}

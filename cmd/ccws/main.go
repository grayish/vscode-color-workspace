package main

import (
	"fmt"
	"os"
)

func main() {
	cmd := rootCmd()
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true
	if err := cmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(errToExit(err))
	}
}

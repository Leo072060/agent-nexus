package main

import (
	"os"

	"agent-nexus-cli/internal/commands"
)

func main() {
	if err := commands.Execute(); err != nil {
		os.Exit(1)
	}
}

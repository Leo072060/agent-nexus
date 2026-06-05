package commands

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

const defaultDBPath = "./agent-nexus.db"

type AppConfig struct {
	DBPath string
}

func NewRootCommand() *cobra.Command {
	cfg := &AppConfig{}

	cmd := &cobra.Command{
		Use:           "agent-nexus",
		Short:         "Agent Nexus command line tools",
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			if cfg.DBPath != "" {
				return
			}

			cfg.DBPath = os.Getenv("AGENT_NEXUS_DB")
			if cfg.DBPath == "" {
				cfg.DBPath = defaultDBPath
			}
		},
	}

	cmd.PersistentFlags().StringVar(&cfg.DBPath, "db", "", "SQLite database path")

	cmd.AddCommand(NewHashCommand())
	cmd.AddCommand(NewRequestCommand(cfg))

	return cmd
}

func Execute() error {
	cmd := NewRootCommand()
	if err := cmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return err
	}

	return nil
}

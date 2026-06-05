package commands

import (
	"fmt"
	"os"

	nexushash "agent-nexus-cli/internal/hash"

	"github.com/spf13/cobra"
)

func NewHashCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "hash",
		Short: "Generate bytes32 hashes for Agent Nexus evidence",
	}

	cmd.AddCommand(newHashTextCommand())
	cmd.AddCommand(newHashFileCommand())

	return cmd
}

func newHashTextCommand() *cobra.Command {
	var value string

	cmd := &cobra.Command{
		Use:   "text",
		Short: "Hash text with Ethereum Keccak256",
		RunE: func(cmd *cobra.Command, args []string) error {
			if value == "" {
				return fmt.Errorf("--value is required")
			}

			fmt.Fprintln(cmd.OutOrStdout(), nexushash.Keccak256Hex([]byte(value)))
			return nil
		},
	}

	cmd.Flags().StringVar(&value, "value", "", "text value to hash")

	return cmd
}

func newHashFileCommand() *cobra.Command {
	var path string

	cmd := &cobra.Command{
		Use:   "file",
		Short: "Hash file contents with Ethereum Keccak256",
		RunE: func(cmd *cobra.Command, args []string) error {
			if path == "" {
				return fmt.Errorf("--path is required")
			}

			contents, err := os.ReadFile(path)
			if err != nil {
				return fmt.Errorf("read file: %w", err)
			}

			fmt.Fprintln(cmd.OutOrStdout(), nexushash.Keccak256Hex(contents))
			return nil
		},
	}

	cmd.Flags().StringVar(&path, "path", "", "file path to hash")

	return cmd
}

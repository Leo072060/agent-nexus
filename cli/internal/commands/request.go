package commands

import (
	"fmt"
	"os"

	nexushash "agent-nexus-cli/internal/hash"
	"agent-nexus-cli/internal/store"

	"github.com/spf13/cobra"
)

func NewRequestCommand(cfg *AppConfig) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "request",
		Short: "Manage buyer request evidence",
	}

	cmd.AddCommand(newRequestCreateCommand(cfg))
	cmd.AddCommand(newRequestGetCommand(cfg))
	cmd.AddCommand(newRequestListCommand(cfg))

	return cmd
}

func newRequestCreateCommand(cfg *AppConfig) *cobra.Command {
	var text string
	var filePath string

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a local buyer request record",
		RunE: func(cmd *cobra.Command, args []string) error {
			if (text == "" && filePath == "") || (text != "" && filePath != "") {
				return fmt.Errorf("exactly one of --text or --file is required")
			}

			content := text
			contentPath := ""
			hashInput := []byte(text)

			if filePath != "" {
				contents, err := os.ReadFile(filePath)
				if err != nil {
					return fmt.Errorf("read file: %w", err)
				}

				content = string(contents)
				contentPath = filePath
				hashInput = contents
			}

			db, err := store.Open(cfg.DBPath)
			if err != nil {
				return err
			}
			defer db.Close()

			request, err := db.CreateRequest(cmd.Context(), store.CreateRequestInput{
				RequestHash: nexushash.Keccak256Hex(hashInput),
				Content:     content,
				ContentPath: contentPath,
			})
			if err != nil {
				return err
			}

			return writeJSON(cmd, request)
		},
	}

	cmd.Flags().StringVar(&text, "text", "", "request text to store and hash")
	cmd.Flags().StringVar(&filePath, "file", "", "request file to store and hash")

	return cmd
}

func newRequestGetCommand(cfg *AppConfig) *cobra.Command {
	var id int64

	cmd := &cobra.Command{
		Use:   "get",
		Short: "Get a local buyer request record",
		RunE: func(cmd *cobra.Command, args []string) error {
			if id <= 0 {
				return fmt.Errorf("--id must be greater than 0")
			}

			db, err := store.Open(cfg.DBPath)
			if err != nil {
				return err
			}
			defer db.Close()

			request, err := db.GetRequest(cmd.Context(), id)
			if err != nil {
				return err
			}

			return writeJSON(cmd, request)
		},
	}

	cmd.Flags().Int64Var(&id, "id", 0, "local request id")

	return cmd
}

func newRequestListCommand(cfg *AppConfig) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List local buyer request records",
		RunE: func(cmd *cobra.Command, args []string) error {
			db, err := store.Open(cfg.DBPath)
			if err != nil {
				return err
			}
			defer db.Close()

			requests, err := db.ListRequests(cmd.Context())
			if err != nil {
				return err
			}

			return writeJSON(cmd, requests)
		},
	}

	return cmd
}

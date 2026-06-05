package commands

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

func writeJSON(cmd *cobra.Command, value any) error {
	encoded, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}

	fmt.Fprintln(cmd.OutOrStdout(), string(encoded))
	return nil
}

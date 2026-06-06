package commands

import (
	"fmt"

	"agent-nexus-cli/internal/store"

	"github.com/ethereum/go-ethereum/common"
	"github.com/spf13/cobra"
)

func NewMarketCommand(cfg *AppConfig) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "market",
		Short: "Manage local market configuration",
	}

	cmd.AddCommand(newMarketAddCommand(cfg))
	cmd.AddCommand(newMarketActivateCommand(cfg))
	cmd.AddCommand(newMarketDeactivateCommand(cfg))
	cmd.AddCommand(newMarketUseCommand(cfg))
	cmd.AddCommand(newMarketActiveCommand(cfg))
	cmd.AddCommand(newMarketGetCommand(cfg))
	cmd.AddCommand(newMarketListCommand(cfg))

	return cmd
}

func newMarketAddCommand(cfg *AppConfig) *cobra.Command {
	var name string
	var rpcURL string
	var marketAddress string

	cmd := &cobra.Command{
		Use:   "add",
		Short: "Add a local market configuration",
		RunE: func(cmd *cobra.Command, args []string) error {
			if name == "" {
				return fmt.Errorf("--name is required")
			}
			if rpcURL == "" {
				return fmt.Errorf("--rpc-url is required")
			}
			if !common.IsHexAddress(marketAddress) {
				return fmt.Errorf("--market-address must be a 20-byte hex address")
			}

			db, err := store.Open(cfg.DBPath)
			if err != nil {
				return err
			}
			defer db.Close()

			market, err := db.CreateMarket(cmd.Context(), store.CreateMarketInput{
				Name:          name,
				RPCURL:        rpcURL,
				MarketAddress: common.HexToAddress(marketAddress).Hex(),
			})
			if err != nil {
				return err
			}

			return writeJSON(cmd, market)
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "local market name")
	cmd.Flags().StringVar(&rpcURL, "rpc-url", "", "market RPC URL")
	cmd.Flags().StringVar(&marketAddress, "market-address", "", "Market contract address")

	return cmd
}

func newMarketActivateCommand(cfg *AppConfig) *cobra.Command {
	var name string

	cmd := &cobra.Command{
		Use:   "activate",
		Short: "Activate a market",
		RunE: func(cmd *cobra.Command, args []string) error {
			if name == "" {
				return fmt.Errorf("--name is required")
			}

			db, err := store.Open(cfg.DBPath)
			if err != nil {
				return err
			}
			defer db.Close()

			market, err := db.ActivateMarket(cmd.Context(), name)
			if err != nil {
				return err
			}

			return writeJSON(cmd, market)
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "local market name")

	return cmd
}

func newMarketDeactivateCommand(cfg *AppConfig) *cobra.Command {
	var name string

	cmd := &cobra.Command{
		Use:   "deactivate",
		Short: "Deactivate a market",
		RunE: func(cmd *cobra.Command, args []string) error {
			if name == "" {
				return fmt.Errorf("--name is required")
			}

			db, err := store.Open(cfg.DBPath)
			if err != nil {
				return err
			}
			defer db.Close()

			market, err := db.DeactivateMarket(cmd.Context(), name)
			if err != nil {
				return err
			}

			return writeJSON(cmd, market)
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "local market name")

	return cmd
}

func newMarketUseCommand(cfg *AppConfig) *cobra.Command {
	var name string

	cmd := &cobra.Command{
		Use:   "use",
		Short: "Activate a market alias",
		RunE: func(cmd *cobra.Command, args []string) error {
			if name == "" {
				return fmt.Errorf("--name is required")
			}

			db, err := store.Open(cfg.DBPath)
			if err != nil {
				return err
			}
			defer db.Close()

			market, err := db.ActivateMarket(cmd.Context(), name)
			if err != nil {
				return err
			}

			return writeJSON(cmd, market)
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "local market name")

	return cmd
}

func newMarketActiveCommand(cfg *AppConfig) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "active",
		Short: "List active markets",
		RunE: func(cmd *cobra.Command, args []string) error {
			db, err := store.Open(cfg.DBPath)
			if err != nil {
				return err
			}
			defer db.Close()

			markets, err := db.ListActiveMarkets(cmd.Context())
			if err != nil {
				return err
			}

			return writeJSON(cmd, markets)
		},
	}

	return cmd
}

func newMarketGetCommand(cfg *AppConfig) *cobra.Command {
	var name string

	cmd := &cobra.Command{
		Use:   "get",
		Short: "Get a market by name",
		RunE: func(cmd *cobra.Command, args []string) error {
			if name == "" {
				return fmt.Errorf("--name is required")
			}

			db, err := store.Open(cfg.DBPath)
			if err != nil {
				return err
			}
			defer db.Close()

			market, err := db.GetMarket(cmd.Context(), name)
			if err != nil {
				return err
			}

			return writeJSON(cmd, market)
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "local market name")

	return cmd
}

func newMarketListCommand(cfg *AppConfig) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List configured markets",
		RunE: func(cmd *cobra.Command, args []string) error {
			db, err := store.Open(cfg.DBPath)
			if err != nil {
				return err
			}
			defer db.Close()

			markets, err := db.ListMarkets(cmd.Context())
			if err != nil {
				return err
			}

			return writeJSON(cmd, markets)
		},
	}

	return cmd
}

package commands

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"agent-nexus-cli/internal/chain"
	nexushash "agent-nexus-cli/internal/hash"
	"agent-nexus-cli/internal/store"
)

import "github.com/spf13/cobra"

type findSellersOutput struct {
	Markets []marketSellersOut `json:"markets"`
}

type marketSellersOut struct {
	Market        string              `json:"market"`
	MarketAddress string              `json:"marketAddress"`
	RPCURL        string              `json:"rpcUrl"`
	Status        string              `json:"status"`
	Error         string              `json:"error,omitempty"`
	Sellers       []verifiedSellerOut `json:"sellers"`
}

type verifiedSellerOut struct {
	Seller              string `json:"seller"`
	SellerURI           string `json:"sellerURI"`
	Price               string `json:"price"`
	ContentURI          string `json:"contentURI"`
	ContentHash         string `json:"contentHash"`
	ContentHashVerified bool   `json:"contentHashVerified"`
	DeliveryTimeout     string `json:"deliveryTimeout"`
}

func NewFindSellersCommand(cfg *AppConfig) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "find-sellers",
		Short: "Find active sellers with verified content hashes",
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

			httpClient := &http.Client{Timeout: 10 * time.Second}
			output := findSellersOutput{Markets: []marketSellersOut{}}

			for _, market := range markets {
				marketOutput := marketSellersOut{
					Market:        market.Name,
					MarketAddress: market.MarketAddress,
					RPCURL:        market.RPCURL,
					Status:        "ok",
					Sellers:       []verifiedSellerOut{},
				}

				marketClient, err := chain.NewMarketClient(cmd.Context(), market.RPCURL, market.MarketAddress)
				if err != nil {
					marketOutput.Status = "error"
					marketOutput.Error = err.Error()
					output.Markets = append(output.Markets, marketOutput)
					continue
				}

				sellerAddresses, err := marketClient.GetSellers(cmd.Context())
				if err != nil {
					marketClient.Close()
					marketOutput.Status = "error"
					marketOutput.Error = err.Error()
					output.Markets = append(output.Markets, marketOutput)
					continue
				}

				for _, sellerAddress := range sellerAddresses {
					seller, err := marketClient.GetSeller(cmd.Context(), sellerAddress)
					if err != nil || !seller.Active {
						continue
					}

					if !isHTTPURI(seller.ContentURI) {
						continue
					}

					body, err := fetchContentBody(httpClient, seller.ContentURI)
					if err != nil {
						continue
					}

					if !strings.EqualFold(nexushash.Keccak256Hex(body), seller.ContentHash) {
						continue
					}

					marketOutput.Sellers = append(marketOutput.Sellers, verifiedSellerOut{
						Seller:              seller.Address.Hex(),
						SellerURI:           seller.SellerURI,
						Price:               seller.Price.String(),
						ContentURI:          seller.ContentURI,
						ContentHash:         seller.ContentHash,
						ContentHashVerified: true,
						DeliveryTimeout:     seller.DeliveryTimeout.String(),
					})
				}

				marketClient.Close()
				output.Markets = append(output.Markets, marketOutput)
			}

			return writeJSON(cmd, output)
		},
	}

	return cmd
}

func isHTTPURI(uri string) bool {
	return strings.HasPrefix(uri, "http://") || strings.HasPrefix(uri, "https://")
}

func fetchContentBody(client *http.Client, uri string) ([]byte, error) {
	resp, err := client.Get(uri)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("unexpected HTTP status: %d", resp.StatusCode)
	}

	return io.ReadAll(resp.Body)
}

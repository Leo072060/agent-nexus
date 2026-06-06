package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"

	"agent-nexus-validator-service/internal/chain"
	"agent-nexus-validator-service/internal/config"
	"agent-nexus-validator-service/internal/httpapi"
	"agent-nexus-validator-service/internal/llm"
	"agent-nexus-validator-service/internal/store"
)

func main() {
	if len(os.Args) < 2 || os.Args[1] != "serve" {
		fmt.Fprintln(os.Stderr, "usage: validator-service serve")
		os.Exit(2)
	}

	if err := serve(context.Background()); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func serve(ctx context.Context) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	db, err := store.Open(cfg.DBPath)
	if err != nil {
		return err
	}
	defer db.Close()

	market, err := chain.NewMarketClient(ctx, cfg.RPCURL, cfg.MarketAddress, cfg.ValidatorPrivateKey)
	if err != nil {
		return err
	}
	defer market.Close()

	decisionClient, err := llm.NewClient(cfg.DeepSeekAPIKey, cfg.DeepSeekBaseURL, cfg.DeepSeekModel)
	if err != nil {
		return err
	}

	server := &http.Server{
		Addr:    cfg.HTTPAddr,
		Handler: httpapi.NewHandler(cfg, market, db, decisionClient),
	}

	log.Printf("validator service listening on %s", cfg.HTTPAddr)
	return server.ListenAndServe()
}

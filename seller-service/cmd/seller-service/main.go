package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"

	"agent-nexus-seller-service/internal/chain"
	"agent-nexus-seller-service/internal/config"
	"agent-nexus-seller-service/internal/httpapi"
	"agent-nexus-seller-service/internal/llm"
	"agent-nexus-seller-service/internal/logging"
	"agent-nexus-seller-service/internal/preflight"
	"agent-nexus-seller-service/internal/store"
	"agent-nexus-seller-service/internal/watcher"
)

func main() {
	if len(os.Args) < 2 || os.Args[1] != "serve" {
		fmt.Fprintln(os.Stderr, "usage: seller-service serve")
		os.Exit(2)
	}

	if err := serve(context.Background()); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func serve(ctx context.Context) error {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	logFile, err := logging.Setup(cfg.LogPath)
	if err != nil {
		return fmt.Errorf("open log file: %w", err)
	}
	defer logFile.Close()

	log.Printf(
		"seller service starting http_addr=%s db_path=%s market_address=%s seller_address=%s llm_script=%s log_path=%s",
		cfg.HTTPAddr,
		cfg.DBPath,
		cfg.MarketAddress.Hex(),
		cfg.SellerAddress.Hex(),
		cfg.LLMScript,
		cfg.LogPath,
	)

	// Initialize database
	db, err := store.Open(cfg.DBPath)
	if err != nil {
		return err
	}
	defer db.Close()

	// Initialize market client
	market, err := chain.NewMarketClient(ctx, cfg.RPCURL, cfg.MarketAddress, cfg.SellerPrivateKey)
	if err != nil {
		return err
	}
	defer market.Close()

	if err := preflight.EnsureSeller(ctx, cfg, market, os.Stdin, os.Stdout); err != nil {
		return err
	}

	generator := llm.NewClient(cfg.LLMScript, cfg.LLMAPIKey, cfg.LLMTimeout)

	// Start HTTP server
	server := &http.Server{
		Addr:    cfg.HTTPAddr,
		Handler: httpapi.NewHandler(cfg, market, db),
	}

	// Start order watcher
	orderWatcher := watcher.New(market, db, generator, cfg.MarketAddress, cfg.SellerAddress, cfg.SellerPrivateKey, cfg.PollInterval)
	go orderWatcher.Run(ctx)

	log.Printf("seller service listening on %s", cfg.HTTPAddr)
	return server.ListenAndServe()
}

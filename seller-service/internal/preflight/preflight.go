package preflight

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"math/big"
	"strings"

	"agent-nexus-seller-service/internal/chain"
	"agent-nexus-seller-service/internal/config"

	"github.com/ethereum/go-ethereum/common"
)

type Market interface {
	GetSeller(ctx context.Context, seller common.Address) (chain.Seller, error)
	RegisterSeller(ctx context.Context, sellerURI string, price *big.Int, contentURI string, contentHash [32]byte, deliveryTimeout *big.Int) (string, error)
	SetSellerURI(ctx context.Context, sellerURI string) (string, error)
	SetProduct(ctx context.Context, price *big.Int, contentURI string, contentHash [32]byte, deliveryTimeout *big.Int) (string, error)
	SetSellerActive(ctx context.Context, active bool) (string, error)
	IsValidatorSupported(ctx context.Context, seller common.Address, validator common.Address) (bool, error)
	AddSupportedValidator(ctx context.Context, validator common.Address) (string, error)
}

type mismatch struct {
	field string
	want  string
	got   string
}

func EnsureSeller(ctx context.Context, cfg config.Config, market Market, in io.Reader, out io.Writer) error {
	seller, err := market.GetSeller(ctx, cfg.SellerAddress)
	if err != nil {
		return fmt.Errorf("read seller from market: %w", err)
	}

	if !seller.Registered {
		printSellerConfig(out, cfg)
		if !confirm(in, out, "Seller is not registered on-chain. Register now? [y/N]: ") {
			return errors.New("seller is not registered on-chain")
		}
		log.Printf("preflight registering seller seller_address=%s", cfg.SellerAddress.Hex())
		txHash, err := market.RegisterSeller(ctx, cfg.SellerURI, cfg.SellerPriceWei, cfg.SellerContentURI, cfg.SellerContentHash, cfg.SellerDeliveryTimeout)
		if err != nil {
			return fmt.Errorf("register seller: %w", err)
		}
		log.Printf("preflight registered seller tx_hash=%s", txHash)
		fmt.Fprintf(out, "Seller registered: %s\n", txHash)
		return ensureValidators(ctx, cfg, market)
	}

	mismatches, err := collectMismatches(ctx, cfg, market, seller)
	if err != nil {
		return err
	}
	if len(mismatches) == 0 {
		log.Printf("preflight seller config matches chain seller_address=%s", cfg.SellerAddress.Hex())
		return nil
	}

	printMismatches(out, mismatches)
	if !confirm(in, out, "On-chain seller config differs. Update now? [y/N]: ") {
		return errors.New("on-chain seller config differs")
	}

	if seller.SellerURI != cfg.SellerURI {
		log.Printf("preflight updating seller uri seller_address=%s", cfg.SellerAddress.Hex())
		txHash, err := market.SetSellerURI(ctx, cfg.SellerURI)
		if err != nil {
			return fmt.Errorf("set seller uri: %w", err)
		}
		log.Printf("preflight updated seller uri tx_hash=%s", txHash)
	}
	if productDiffers(cfg, seller) {
		log.Printf("preflight updating seller product seller_address=%s", cfg.SellerAddress.Hex())
		txHash, err := market.SetProduct(ctx, cfg.SellerPriceWei, cfg.SellerContentURI, cfg.SellerContentHash, cfg.SellerDeliveryTimeout)
		if err != nil {
			return fmt.Errorf("set product: %w", err)
		}
		log.Printf("preflight updated seller product tx_hash=%s", txHash)
	}
	if !seller.Active {
		log.Printf("preflight activating seller seller_address=%s", cfg.SellerAddress.Hex())
		txHash, err := market.SetSellerActive(ctx, true)
		if err != nil {
			return fmt.Errorf("set seller active: %w", err)
		}
		log.Printf("preflight activated seller tx_hash=%s", txHash)
	}
	if err := ensureValidators(ctx, cfg, market); err != nil {
		return err
	}

	return nil
}

func collectMismatches(ctx context.Context, cfg config.Config, market Market, seller chain.Seller) ([]mismatch, error) {
	var mismatches []mismatch
	if !seller.Active {
		mismatches = append(mismatches, mismatch{"active", "true", "false"})
	}
	if seller.SellerURI != cfg.SellerURI {
		mismatches = append(mismatches, mismatch{"sellerURI", cfg.SellerURI, seller.SellerURI})
	}
	if seller.Price.Cmp(cfg.SellerPriceWei) != 0 {
		mismatches = append(mismatches, mismatch{"price", cfg.SellerPriceWei.String(), seller.Price.String()})
	}
	if seller.ContentURI != cfg.SellerContentURI {
		mismatches = append(mismatches, mismatch{"contentURI", cfg.SellerContentURI, seller.ContentURI})
	}
	if seller.ContentHash != cfg.SellerContentHash {
		mismatches = append(mismatches, mismatch{"contentHash", bytes32Hex(cfg.SellerContentHash), bytes32Hex(seller.ContentHash)})
	}
	if seller.DeliveryTimeout.Cmp(cfg.SellerDeliveryTimeout) != 0 {
		mismatches = append(mismatches, mismatch{"deliveryTimeout", cfg.SellerDeliveryTimeout.String(), seller.DeliveryTimeout.String()})
	}
	for _, validator := range cfg.SupportedValidators {
		supported, err := market.IsValidatorSupported(ctx, cfg.SellerAddress, validator)
		if err != nil {
			return nil, fmt.Errorf("check validator support %s: %w", validator.Hex(), err)
		}
		if !supported {
			mismatches = append(mismatches, mismatch{"supportedValidator", validator.Hex(), "missing"})
		}
	}

	return mismatches, nil
}

func ensureValidators(ctx context.Context, cfg config.Config, market Market) error {
	for _, validator := range cfg.SupportedValidators {
		supported, err := market.IsValidatorSupported(ctx, cfg.SellerAddress, validator)
		if err != nil {
			return fmt.Errorf("check validator support %s: %w", validator.Hex(), err)
		}
		if supported {
			continue
		}
		log.Printf("preflight adding supported validator seller_address=%s validator=%s", cfg.SellerAddress.Hex(), validator.Hex())
		txHash, err := market.AddSupportedValidator(ctx, validator)
		if err != nil {
			return fmt.Errorf("add supported validator %s: %w", validator.Hex(), err)
		}
		log.Printf("preflight added supported validator validator=%s tx_hash=%s", validator.Hex(), txHash)
	}
	return nil
}

func productDiffers(cfg config.Config, seller chain.Seller) bool {
	return seller.Price.Cmp(cfg.SellerPriceWei) != 0 ||
		seller.ContentURI != cfg.SellerContentURI ||
		seller.ContentHash != cfg.SellerContentHash ||
		seller.DeliveryTimeout.Cmp(cfg.SellerDeliveryTimeout) != 0
}

func confirm(in io.Reader, out io.Writer, prompt string) bool {
	fmt.Fprint(out, prompt)
	line, _ := bufio.NewReader(in).ReadString('\n')
	return strings.EqualFold(strings.TrimSpace(line), "y")
}

func printSellerConfig(out io.Writer, cfg config.Config) {
	fmt.Fprintf(out, "Seller URI: %s\n", cfg.SellerURI)
	fmt.Fprintf(out, "Price wei: %s\n", cfg.SellerPriceWei.String())
	fmt.Fprintf(out, "Content URI: %s\n", cfg.SellerContentURI)
	fmt.Fprintf(out, "Content hash: %s\n", bytes32Hex(cfg.SellerContentHash))
	fmt.Fprintf(out, "Delivery timeout: %s\n", cfg.SellerDeliveryTimeout.String())
}

func printMismatches(out io.Writer, mismatches []mismatch) {
	fmt.Fprintln(out, "On-chain seller config mismatches:")
	for _, item := range mismatches {
		fmt.Fprintf(out, "- %s: want %s, got %s\n", item.field, item.want, item.got)
	}
}

func bytes32Hex(value [32]byte) string {
	return "0x" + common.Bytes2Hex(value[:])
}

package preflight

import (
	"context"
	"errors"
	"math/big"
	"strings"
	"testing"

	"agent-nexus-seller-service/internal/chain"
	"agent-nexus-seller-service/internal/config"

	"github.com/ethereum/go-ethereum/common"
)

type recordingMarket struct {
	seller              chain.Seller
	supportedValidators map[common.Address]bool
	calls               []string
	registerErr         error
}

func (r *recordingMarket) GetSeller(context.Context, common.Address) (chain.Seller, error) {
	return r.seller, nil
}

func (r *recordingMarket) RegisterSeller(context.Context, string, *big.Int, string, [32]byte, *big.Int) (string, error) {
	r.calls = append(r.calls, "registerSeller")
	if r.registerErr != nil {
		return "", r.registerErr
	}
	r.seller.Registered = true
	r.seller.Active = true
	return "0xregister", nil
}

func TestEnsureSellerStopsWhenRegisterFails(t *testing.T) {
	cfg := testConfig()
	market := &recordingMarket{
		supportedValidators: map[common.Address]bool{},
		registerErr:         errors.New("tx failed"),
	}

	err := EnsureSeller(context.Background(), cfg, market, strings.NewReader("y\n"), &strings.Builder{})
	if err == nil || !strings.Contains(err.Error(), "register seller") {
		t.Fatalf("expected register error, got %v", err)
	}
	if len(market.calls) != 1 || market.calls[0] != "registerSeller" {
		t.Fatalf("calls mismatch: %#v", market.calls)
	}
}

func (r *recordingMarket) SetSellerURI(context.Context, string) (string, error) {
	r.calls = append(r.calls, "setSellerURI")
	return "0xuri", nil
}

func (r *recordingMarket) SetProduct(context.Context, *big.Int, string, [32]byte, *big.Int) (string, error) {
	r.calls = append(r.calls, "setProduct")
	return "0xproduct", nil
}

func (r *recordingMarket) SetSellerActive(context.Context, bool) (string, error) {
	r.calls = append(r.calls, "setSellerActive")
	return "0xactive", nil
}

func (r *recordingMarket) IsValidatorSupported(_ context.Context, _ common.Address, validator common.Address) (bool, error) {
	return r.supportedValidators[validator], nil
}

func (r *recordingMarket) AddSupportedValidator(_ context.Context, validator common.Address) (string, error) {
	r.calls = append(r.calls, "addSupportedValidator:"+validator.Hex())
	r.supportedValidators[validator] = true
	return "0xvalidator", nil
}

func TestEnsureSellerUnregisteredDeclined(t *testing.T) {
	cfg := testConfig()
	market := &recordingMarket{supportedValidators: map[common.Address]bool{}}

	if err := EnsureSeller(context.Background(), cfg, market, strings.NewReader("n\n"), &strings.Builder{}); err == nil {
		t.Fatal("expected error")
	}
	if len(market.calls) != 0 {
		t.Fatalf("expected no calls, got %#v", market.calls)
	}
}

func TestEnsureSellerUnregisteredRegistersAndAddsValidators(t *testing.T) {
	cfg := testConfig()
	market := &recordingMarket{supportedValidators: map[common.Address]bool{}}

	if err := EnsureSeller(context.Background(), cfg, market, strings.NewReader("y\n"), &strings.Builder{}); err != nil {
		t.Fatal(err)
	}
	want := []string{"registerSeller", "addSupportedValidator:" + cfg.SupportedValidators[0].Hex()}
	if strings.Join(market.calls, ",") != strings.Join(want, ",") {
		t.Fatalf("calls mismatch: %#v", market.calls)
	}
}

func TestEnsureSellerMatchingDoesNothing(t *testing.T) {
	cfg := testConfig()
	market := &recordingMarket{
		seller:              matchingSeller(cfg),
		supportedValidators: map[common.Address]bool{cfg.SupportedValidators[0]: true},
	}

	if err := EnsureSeller(context.Background(), cfg, market, strings.NewReader(""), &strings.Builder{}); err != nil {
		t.Fatal(err)
	}
	if len(market.calls) != 0 {
		t.Fatalf("expected no calls, got %#v", market.calls)
	}
}

func TestEnsureSellerUpdatesMismatches(t *testing.T) {
	cfg := testConfig()
	market := &recordingMarket{
		seller: chain.Seller{
			Registered:      true,
			Active:          false,
			SellerURI:       "old",
			Price:           big.NewInt(1),
			ContentURI:      "old-content",
			ContentHash:     [32]byte{9},
			DeliveryTimeout: big.NewInt(1),
		},
		supportedValidators: map[common.Address]bool{},
	}

	if err := EnsureSeller(context.Background(), cfg, market, strings.NewReader("y\n"), &strings.Builder{}); err != nil {
		t.Fatal(err)
	}
	calls := strings.Join(market.calls, ",")
	for _, want := range []string{"setSellerURI", "setProduct", "setSellerActive", "addSupportedValidator:" + cfg.SupportedValidators[0].Hex()} {
		if !strings.Contains(calls, want) {
			t.Fatalf("expected %s in calls %#v", want, market.calls)
		}
	}
}

func testConfig() config.Config {
	contentHash := [32]byte{1}
	validator := common.HexToAddress("0x3333333333333333333333333333333333333333")
	return config.Config{
		SellerAddress:         common.HexToAddress("0x1111111111111111111111111111111111111111"),
		SellerURI:             "https://seller.example/agent-nexus",
		SellerPriceWei:        big.NewInt(100),
		SellerContentURI:      "ipfs://seller/product",
		SellerContentHash:     contentHash,
		SellerDeliveryTimeout: big.NewInt(3600),
		SupportedValidators:   []common.Address{validator},
	}
}

func matchingSeller(cfg config.Config) chain.Seller {
	return chain.Seller{
		Registered:      true,
		Active:          true,
		SellerURI:       cfg.SellerURI,
		Price:           cfg.SellerPriceWei,
		ContentURI:      cfg.SellerContentURI,
		ContentHash:     cfg.SellerContentHash,
		DeliveryTimeout: cfg.SellerDeliveryTimeout,
	}
}

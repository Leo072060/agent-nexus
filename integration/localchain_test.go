package integration

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	gethcrypto "github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
)

const (
	ownerKeyHex     = "ac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80"
	buyerKeyHex     = "59c6995e998f97a5a0044966f094538a9a7a5cc09bf4ed85a728c36cfcfb0329"
	sellerKeyHex    = "0000000000000000000000000000000000000000000000000000000000000003"
	validatorKeyHex = "0000000000000000000000000000000000000000000000000000000000000004"
	outsiderKeyHex  = "0000000000000000000000000000000000000000000000000000000000000005"

	statusPendingValidator    = uint8(2)
	statusCreated             = uint8(3)
	statusDeliveryCommitted   = uint8(4)
	statusDisputed            = uint8(5)
	statusReleased            = uint8(6)
	statusResolvedToSeller    = uint8(9)
	statusResolvedToBuyer     = uint8(10)
	statusDisputeTimeoutSplit = uint8(11)
)

var (
	price           = big.NewInt(1_000_000_000_000_000_000)
	validatorFee    = big.NewInt(100_000_000_000_000_000)
	deliveryTimeout = big.NewInt(3600)
	responseTimeout = big.NewInt(3600)
	approvalTimeout = big.NewInt(600)
)

type artifact struct {
	ABI      json.RawMessage `json:"abi"`
	Bytecode struct {
		Object string `json:"object"`
	} `json:"bytecode"`
}

type chainHarness struct {
	t             *testing.T
	ctx           context.Context
	root          string
	rpcURL        string
	client        *ethclient.Client
	chainID       *big.Int
	parsedABI     abi.ABI
	marketAddress common.Address
	owner         account
	buyer         account
	seller        account
	validator     account
	outsider      account
}

type account struct {
	key     *ecdsa.PrivateKey
	address common.Address
}

type orderView struct {
	Buyer            common.Address
	Seller           common.Address
	Validator        common.Address
	Amount           *big.Int
	ValidatorFee     *big.Int
	ValidatorBond    *big.Int
	ListingHash      [32]byte
	RequestHash      [32]byte
	DeliveryHash     [32]byte
	ResolutionHash   [32]byte
	CreatedAt        *big.Int
	ApprovalDeadline *big.Int
	DeliveryDeadline *big.Int
	ResponseDeadline *big.Int
	Status           uint8
}

func TestLocalChainContractLifecycle(t *testing.T) {
	h := newHarness(t)
	h.registerSellerAndValidator("http://seller", "http://validator")

	orderID := h.createOrder(h.buyer, h.seller.address, h.validator.address, hash32("request-ok"), approvalTimeout, new(big.Int).Add(price, validatorFee))
	h.tx(h.seller, big.NewInt(0), "confirmAsSeller", orderID)
	h.tx(h.validator, price, "confirmAsValidator", orderID)
	if bond := h.getOrder(orderID).ValidatorBond; bond.Cmp(price) != 0 {
		t.Fatalf("validator bond got %s want %s", bond, price)
	}
	h.tx(h.seller, big.NewInt(0), "commitDelivery", orderID, hash32("delivery-ok"))
	validatorBeforeAccept := h.balance(h.validator.address)
	h.tx(h.buyer, big.NewInt(0), "acceptDelivery", orderID)
	h.assertOrderStatus(orderID, statusReleased)
	h.assertBalanceDelta(h.validator.address, validatorBeforeAccept, price)

	sellerDisputeID := h.createCommittedOrder(hash32("seller-dispute"), hash32("seller-delivery"))
	h.tx(h.seller, big.NewInt(0), "openDispute", sellerDisputeID)
	validatorBeforeSellerResolution := h.balance(h.validator.address)
	sellerResolutionReceipt := h.tx(h.validator, big.NewInt(0), "resolveDispute", sellerDisputeID, true, hash32("seller-wins"))
	h.assertOrderStatus(sellerDisputeID, statusResolvedToSeller)
	h.assertBalanceDeltaWithGas(h.validator.address, validatorBeforeSellerResolution, sellerResolutionReceipt, new(big.Int).Add(validatorFee, price))

	buyerDisputeID := h.createCommittedOrder(hash32("buyer-dispute"), hash32("bad-delivery"))
	h.tx(h.buyer, big.NewInt(0), "openDispute", buyerDisputeID)
	validatorBeforeBuyerResolution := h.balance(h.validator.address)
	buyerResolutionReceipt := h.tx(h.validator, big.NewInt(0), "resolveDispute", buyerDisputeID, false, hash32("buyer-wins"))
	h.assertOrderStatus(buyerDisputeID, statusResolvedToBuyer)
	h.assertBalanceDeltaWithGas(h.validator.address, validatorBeforeBuyerResolution, buyerResolutionReceipt, new(big.Int).Add(validatorFee, price))
}

func TestLocalChainTimeoutsAndFailures(t *testing.T) {
	h := newHarness(t)
	h.registerSellerAndValidator("http://seller", "http://validator")

	approvalID := h.createOrder(h.buyer, h.seller.address, h.validator.address, hash32("approval-timeout"), approvalTimeout, new(big.Int).Add(price, validatorFee))
	h.increaseTime(new(big.Int).Add(approvalTimeout, big.NewInt(1)))
	h.tx(h.outsider, big.NewInt(0), "refundIfApprovalExpired", approvalID)

	deliveryID := h.createOrder(h.buyer, h.seller.address, h.validator.address, hash32("delivery-timeout"), approvalTimeout, new(big.Int).Add(price, validatorFee))
	h.tx(h.seller, big.NewInt(0), "confirmAsSeller", deliveryID)
	h.expectTxRevert(h.validator, "confirmAsValidator", deliveryID)
	h.expectTxRevertWithValue(h.validator, big.NewInt(1), "confirmAsValidator", deliveryID)
	h.tx(h.validator, price, "confirmAsValidator", deliveryID)
	h.increaseTime(new(big.Int).Add(deliveryTimeout, big.NewInt(1)))
	validatorBeforeDeliveryRefund := h.balance(h.validator.address)
	h.tx(h.outsider, big.NewInt(0), "refundIfDeliveryExpired", deliveryID)
	h.assertBalanceDelta(h.validator.address, validatorBeforeDeliveryRefund, new(big.Int).Add(validatorFee, price))

	disputeAfterExpiryID := h.createOrder(h.buyer, h.seller.address, h.validator.address, hash32("delivery-timeout-dispute"), approvalTimeout, new(big.Int).Add(price, validatorFee))
	h.tx(h.seller, big.NewInt(0), "confirmAsSeller", disputeAfterExpiryID)
	h.tx(h.validator, price, "confirmAsValidator", disputeAfterExpiryID)
	h.increaseTime(new(big.Int).Add(deliveryTimeout, big.NewInt(1)))
	h.tx(h.buyer, big.NewInt(0), "openDispute", disputeAfterExpiryID)
	h.assertOrderStatus(disputeAfterExpiryID, statusDisputed)
	h.expectTxRevert(h.outsider, "splitIfResolutionExpired", disputeAfterExpiryID)
	h.increaseTime(new(big.Int).Add(responseTimeout, big.NewInt(1)))
	h.expectTxRevert(h.validator, "resolveDispute", disputeAfterExpiryID, false, hash32("too-late"))
	buyerBeforeSplit := h.balance(h.buyer.address)
	h.tx(h.seller, big.NewInt(0), "splitIfResolutionExpired", disputeAfterExpiryID)
	h.assertOrderStatus(disputeAfterExpiryID, statusDisputeTimeoutSplit)
	h.assertBalanceDelta(h.buyer.address, buyerBeforeSplit, new(big.Int).Add(validatorFee, price))
	h.expectTxRevert(h.validator, "resolveDispute", disputeAfterExpiryID, false, hash32("too-late-again"))

	h.expectTxRevert(h.outsider, "confirmAsSeller", h.createOrder(h.buyer, h.seller.address, h.validator.address, hash32("auth"), approvalTimeout, new(big.Int).Add(price, validatorFee)))
	h.expectTxRevert(h.buyer, "createOrder", h.seller.address, h.validator.address, [32]byte{}, approvalTimeout)
	h.expectTxRevertWithValue(h.buyer, big.NewInt(1), "createOrder", h.seller.address, h.validator.address, hash32("bad-payment"), approvalTimeout)

	h.tx(h.seller, big.NewInt(0), "setSellerActive", false)
	h.expectTxRevertWithValue(h.buyer, new(big.Int).Add(price, validatorFee), "createOrder", h.seller.address, h.validator.address, hash32("inactive-seller"), approvalTimeout)
	h.tx(h.seller, big.NewInt(0), "setSellerActive", true)
	h.tx(h.validator, big.NewInt(0), "setValidatorActive", false)
	h.expectTxRevertWithValue(h.buyer, new(big.Int).Add(price, validatorFee), "createOrder", h.seller.address, h.validator.address, hash32("inactive-validator"), approvalTimeout)
}

func TestSellerServiceDeliveryFlow(t *testing.T) {
	h := newHarness(t)
	sellerURL := "http://127.0.0.1:" + freePort(t)
	h.registerSellerAndValidator(sellerURL, "http://validator")

	sellerAddr := strings.TrimPrefix(sellerURL, "http://127.0.0.1")
	sellerDB := filepath.Join(t.TempDir(), "seller.db")
	sellerLLMScript := writeSellerLLMScript(t)
	sellerProc := startProcess(t, "../seller-service", []string{"go", "run", "./cmd/seller-service", "serve"}, append(os.Environ(),
		"SELLER_RPC_URL="+h.rpcURL,
		"SELLER_MARKET_ADDRESS="+h.marketAddress.Hex(),
		"SELLER_PRIVATE_KEY="+sellerKeyHex,
		"SELLER_BASE_URL="+sellerURL,
		"SELLER_URI="+sellerURL,
		"SELLER_PRICE_WEI="+price.String(),
		"SELLER_CONTENT_URI=http://content",
		"SELLER_CONTENT_HASH="+hexHash(hash32("content")),
		"SELLER_DELIVERY_TIMEOUT="+deliveryTimeout.String(),
		"SELLER_SUPPORTED_VALIDATORS="+h.validator.address.Hex(),
		"SELLER_SERVICE_ID=integration-seller",
		"SELLER_SERVICE_NAME=Integration Seller",
		"SELLER_SERVICE_DESCRIPTION=Integration test seller service",
		"SELLER_DB_PATH="+sellerDB,
		"SELLER_HTTP_ADDR="+sellerAddr,
		"SELLER_POLL_INTERVAL=200ms",
		"SELLER_LLM_SCRIPT="+sellerLLMScript,
		"SELLER_LLM_API_KEY=integration-test-key",
	))
	defer sellerProc.stop()
	waitHTTP(t, sellerURL+"/health")

	requestBody := "please deliver a concise test artifact"
	requestHash := hash32(requestBody)
	orderID := h.createOrder(h.buyer, h.seller.address, h.validator.address, requestHash, approvalTimeout, new(big.Int).Add(price, validatorFee))
	orderSig := signPersonal(t, h.buyer.key, orderRequestMessage(h.marketAddress, orderID, hexHash(requestHash)))

	postJSON(t, sellerURL+"/agent-nexus/request", map[string]string{
		"marketAddress": h.marketAddress.Hex(),
		"orderId":       orderID.String(),
		"request":       requestBody,
		"signature":     orderSig,
	}, http.StatusOK)
	h.waitOrderStatus(orderID, statusPendingValidator, 5*time.Second)

	h.tx(h.validator, price, "confirmAsValidator", orderID)
	h.waitOrderStatus(orderID, statusDeliveryCommitted, 10*time.Second)

	deliverySig := signPersonal(t, h.buyer.key, deliveryRequestMessage(h.marketAddress, orderID))
	body := postJSONRaw(t, sellerURL+"/agent-nexus/delivery", map[string]string{
		"marketAddress": h.marketAddress.Hex(),
		"orderId":       orderID.String(),
		"signature":     deliverySig,
	}, http.StatusOK)

	order := h.getOrder(orderID)
	if got := gethcrypto.Keccak256Hash(body); got != common.Hash(order.DeliveryHash) {
		t.Fatalf("delivery hash mismatch: got %s want %s", got.Hex(), common.Hash(order.DeliveryHash).Hex())
	}
}

func TestValidatorServiceWithScript(t *testing.T) {
	h := newHarness(t)
	validatorURL := "http://127.0.0.1:" + freePort(t)
	h.registerSellerAndValidator("http://seller", validatorURL)

	validatorAddr := strings.TrimPrefix(validatorURL, "http://127.0.0.1")
	validatorDB := filepath.Join(t.TempDir(), "validator.db")
	llmScript := writeValidatorLLMScript(t)
	validatorProc := startProcess(t, "../validator-service", []string{"go", "run", "./cmd/validator-service", "serve"}, append(os.Environ(),
		"VALIDATOR_RPC_URL="+h.rpcURL,
		"VALIDATOR_MARKET_ADDRESS="+h.marketAddress.Hex(),
		"VALIDATOR_PRIVATE_KEY="+validatorKeyHex,
		"VALIDATOR_BASE_URL="+validatorURL,
		"VALIDATOR_DB_PATH="+validatorDB,
		"VALIDATOR_HTTP_ADDR="+validatorAddr,
		"VALIDATOR_LLM_SCRIPT="+llmScript,
		"VALIDATOR_LLM_API_KEY=integration-test-key",
		"VALIDATOR_LLM_TIMEOUT=5s",
	))
	defer validatorProc.stop()
	waitHTTP(t, validatorURL+"/health")

	requestBody := "Return exactly the word PASS."
	deliveryBody := "PASS"
	disputeBody := "The delivery should be accepted if it exactly returns PASS."
	orderID := h.createCommittedOrder(hash32(requestBody), hash32(deliveryBody))
	h.tx(h.buyer, big.NewInt(0), "openDispute", orderID)

	requestHash := hexHash(hash32(requestBody))
	deliveryHash := hexHash(hash32(deliveryBody))
	disputeHash := hexHash(hash32(disputeBody))
	signature := signPersonal(t, h.buyer.key, disputeEvidenceMessage(h.marketAddress, orderID, requestHash, deliveryHash, disputeHash))

	respBody, status := postJSONWithStatus(t, validatorURL+"/agent-nexus/disputes", map[string]string{
		"marketAddress": h.marketAddress.Hex(),
		"orderId":       orderID.String(),
		"buyerAddress":  h.buyer.address.Hex(),
		"request":       requestBody,
		"delivery":      deliveryBody,
		"dispute":       disputeBody,
		"signature":     signature,
	})
	if status != http.StatusOK {
		t.Fatalf("validator dispute status=%d body=%s", status, string(respBody))
	}

	order := h.getOrder(orderID)
	if order.Status != statusResolvedToSeller && order.Status != statusResolvedToBuyer {
		t.Fatalf("expected resolved order, got status %d", order.Status)
	}
	if order.ResolutionHash == ([32]byte{}) {
		t.Fatal("expected non-zero resolution hash")
	}
}

func newHarness(t *testing.T) *chainHarness {
	t.Helper()
	root, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}

	build := exec.Command("forge", "build")
	build.Dir = root
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("forge build: %v\n%s", err, string(out))
	}

	port := freePort(t)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	anvil := exec.CommandContext(ctx, "anvil", "--host", "127.0.0.1", "--port", port, "--silent")
	anvil.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	var anvilOutput bytes.Buffer
	anvil.Stdout = &anvilOutput
	anvil.Stderr = &anvilOutput
	if err := anvil.Start(); err != nil {
		t.Fatalf("start anvil: %v", err)
	}
	t.Cleanup(func() {
		stopCmd(anvil, 2*time.Second)
	})

	rpcURL := "http://127.0.0.1:" + port
	client := waitRPC(t, rpcURL)
	chainID, err := client.ChainID(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	h := &chainHarness{
		t:         t,
		ctx:       context.Background(),
		root:      root,
		rpcURL:    rpcURL,
		client:    client,
		chainID:   chainID,
		owner:     mustAccount(t, ownerKeyHex),
		buyer:     mustAccount(t, buyerKeyHex),
		seller:    mustAccount(t, sellerKeyHex),
		validator: mustAccount(t, validatorKeyHex),
		outsider:  mustAccount(t, outsiderKeyHex),
	}
	t.Cleanup(client.Close)
	h.loadArtifactAndDeploy()
	h.fundTestAccounts()
	return h
}

func (h *chainHarness) loadArtifactAndDeploy() {
	h.t.Helper()
	raw, err := os.ReadFile(filepath.Join(h.root, "out", "Market.sol", "Market.json"))
	if err != nil {
		h.t.Fatal(err)
	}
	var art artifact
	if err := json.Unmarshal(raw, &art); err != nil {
		h.t.Fatal(err)
	}
	parsed, err := abi.JSON(bytes.NewReader(art.ABI))
	if err != nil {
		h.t.Fatal(err)
	}
	bytecode := common.FromHex(art.Bytecode.Object)
	constructor, err := parsed.Pack("", "agent-nexus-local")
	if err != nil {
		h.t.Fatal(err)
	}
	receipt := h.send(h.owner, common.Address{}, big.NewInt(0), append(bytecode, constructor...))
	if receipt.ContractAddress == (common.Address{}) {
		h.t.Fatal("deployment did not return contract address")
	}
	h.parsedABI = parsed
	h.marketAddress = receipt.ContractAddress
}

func (h *chainHarness) registerSellerAndValidator(sellerURI string, validatorURI string) {
	h.tx(h.seller, big.NewInt(0), "registerSeller", sellerURI, price, "http://content", hash32("content"), deliveryTimeout)
	h.tx(h.validator, big.NewInt(0), "registerValidator", validatorURI, validatorFee, responseTimeout)
	h.tx(h.seller, big.NewInt(0), "addSupportedValidator", h.validator.address)
}

func (h *chainHarness) fundTestAccounts() {
	h.t.Helper()
	amount := big.NewInt(0).Mul(big.NewInt(10), big.NewInt(1_000_000_000_000_000_000))
	h.send(h.owner, h.buyer.address, amount, nil)
	h.send(h.owner, h.seller.address, amount, nil)
	h.send(h.owner, h.validator.address, amount, nil)
	h.send(h.owner, h.outsider.address, amount, nil)
}

func (h *chainHarness) createCommittedOrder(requestHash [32]byte, deliveryHash [32]byte) *big.Int {
	orderID := h.createOrder(h.buyer, h.seller.address, h.validator.address, requestHash, approvalTimeout, new(big.Int).Add(price, validatorFee))
	h.tx(h.seller, big.NewInt(0), "confirmAsSeller", orderID)
	h.tx(h.validator, price, "confirmAsValidator", orderID)
	h.tx(h.seller, big.NewInt(0), "commitDelivery", orderID, deliveryHash)
	return orderID
}

func (h *chainHarness) createOrder(from account, seller common.Address, validator common.Address, requestHash [32]byte, approvalTimeout *big.Int, value *big.Int) *big.Int {
	h.tx(from, value, "createOrder", seller, validator, requestHash, approvalTimeout)
	count := h.callBig("getOrderCount")
	if count.Sign() <= 0 {
		h.t.Fatal("expected positive order count")
	}
	return count
}

func (h *chainHarness) tx(from account, value *big.Int, method string, args ...any) *types.Receipt {
	h.t.Helper()
	data, err := h.parsedABI.Pack(method, args...)
	if err != nil {
		h.t.Fatalf("pack %s: %v", method, err)
	}
	return h.send(from, h.marketAddress, value, data)
}

func (h *chainHarness) send(from account, to common.Address, value *big.Int, data []byte) *types.Receipt {
	h.t.Helper()
	nonce, err := h.client.PendingNonceAt(h.ctx, from.address)
	if err != nil {
		h.t.Fatal(err)
	}
	gasPrice, err := h.client.SuggestGasPrice(h.ctx)
	if err != nil {
		h.t.Fatal(err)
	}
	msg := ethereum.CallMsg{From: from.address, Data: data, Value: value}
	var tx *types.Transaction
	if to != (common.Address{}) {
		msg.To = &to
		gas, err := h.client.EstimateGas(h.ctx, msg)
		if err != nil {
			h.t.Fatalf("estimate gas: %v", err)
		}
		tx = types.NewTransaction(nonce, to, value, gas, gasPrice, data)
	} else {
		gas, err := h.client.EstimateGas(h.ctx, msg)
		if err != nil {
			h.t.Fatalf("estimate deployment gas: %v", err)
		}
		tx = types.NewContractCreation(nonce, value, gas, gasPrice, data)
	}
	signed, err := types.SignTx(tx, types.LatestSignerForChainID(h.chainID), from.key)
	if err != nil {
		h.t.Fatal(err)
	}
	if err := h.client.SendTransaction(h.ctx, signed); err != nil {
		h.t.Fatal(err)
	}
	receipt, err := waitReceipt(h.ctx, h.client, signed.Hash(), 10*time.Second)
	if err != nil {
		h.t.Fatal(err)
	}
	if receipt.Status != types.ReceiptStatusSuccessful {
		h.t.Fatalf("transaction reverted: %s", signed.Hash().Hex())
	}
	return receipt
}

func (h *chainHarness) expectTxRevert(from account, method string, args ...any) {
	h.expectTxRevertWithValue(from, big.NewInt(0), method, args...)
}

func (h *chainHarness) expectTxRevertWithValue(from account, value *big.Int, method string, args ...any) {
	h.t.Helper()
	data, err := h.parsedABI.Pack(method, args...)
	if err != nil {
		h.t.Fatalf("pack %s: %v", method, err)
	}
	_, err = h.client.EstimateGas(h.ctx, ethereum.CallMsg{
		From:  from.address,
		To:    &h.marketAddress,
		Value: value,
		Data:  data,
	})
	if err == nil {
		h.t.Fatalf("expected %s to revert", method)
	}
}

func (h *chainHarness) getOrder(orderID *big.Int) orderView {
	h.t.Helper()
	data, err := h.parsedABI.Pack("getOrder", orderID)
	if err != nil {
		h.t.Fatal(err)
	}
	result, err := h.client.CallContract(h.ctx, ethereum.CallMsg{To: &h.marketAddress, Data: data}, nil)
	if err != nil {
		h.t.Fatal(err)
	}
	outputs, err := h.parsedABI.Unpack("getOrder", result)
	if err != nil {
		h.t.Fatal(err)
	}
	value := reflect.ValueOf(outputs[0])
	return orderView{
		Buyer:            value.FieldByName("Buyer").Interface().(common.Address),
		Seller:           value.FieldByName("Seller").Interface().(common.Address),
		Validator:        value.FieldByName("Validator").Interface().(common.Address),
		Amount:           value.FieldByName("Amount").Interface().(*big.Int),
		ValidatorFee:     value.FieldByName("ValidatorFee").Interface().(*big.Int),
		ValidatorBond:    value.FieldByName("ValidatorBond").Interface().(*big.Int),
		ListingHash:      value.FieldByName("ListingHash").Interface().([32]byte),
		RequestHash:      value.FieldByName("RequestHash").Interface().([32]byte),
		DeliveryHash:     value.FieldByName("DeliveryHash").Interface().([32]byte),
		ResolutionHash:   value.FieldByName("ResolutionHash").Interface().([32]byte),
		CreatedAt:        value.FieldByName("CreatedAt").Interface().(*big.Int),
		ApprovalDeadline: value.FieldByName("ApprovalDeadline").Interface().(*big.Int),
		DeliveryDeadline: value.FieldByName("DeliveryDeadline").Interface().(*big.Int),
		ResponseDeadline: value.FieldByName("ResponseDeadline").Interface().(*big.Int),
		Status:           value.FieldByName("Status").Interface().(uint8),
	}
}

func (h *chainHarness) callBig(method string, args ...any) *big.Int {
	h.t.Helper()
	data, err := h.parsedABI.Pack(method, args...)
	if err != nil {
		h.t.Fatal(err)
	}
	result, err := h.client.CallContract(h.ctx, ethereum.CallMsg{To: &h.marketAddress, Data: data}, nil)
	if err != nil {
		h.t.Fatal(err)
	}
	outputs, err := h.parsedABI.Unpack(method, result)
	if err != nil {
		h.t.Fatal(err)
	}
	return outputs[0].(*big.Int)
}

func (h *chainHarness) assertOrderStatus(orderID *big.Int, want uint8) {
	h.t.Helper()
	got := h.getOrder(orderID).Status
	if got != want {
		h.t.Fatalf("order %s status got %d want %d", orderID, got, want)
	}
}

func (h *chainHarness) balance(address common.Address) *big.Int {
	h.t.Helper()
	balance, err := h.client.BalanceAt(h.ctx, address, nil)
	if err != nil {
		h.t.Fatal(err)
	}
	return balance
}

func (h *chainHarness) assertBalanceDelta(address common.Address, before *big.Int, wantDelta *big.Int) {
	h.t.Helper()
	after := h.balance(address)
	gotDelta := new(big.Int).Sub(after, before)
	if gotDelta.Cmp(wantDelta) != 0 {
		h.t.Fatalf("balance delta for %s got %s want %s", address.Hex(), gotDelta, wantDelta)
	}
}

func (h *chainHarness) assertBalanceDeltaWithGas(address common.Address, before *big.Int, receipt *types.Receipt, wantDelta *big.Int) {
	h.t.Helper()
	after := h.balance(address)
	gotDelta := new(big.Int).Sub(after, before)
	gasCost := new(big.Int).Mul(new(big.Int).SetUint64(receipt.GasUsed), receipt.EffectiveGasPrice)
	gotDelta.Add(gotDelta, gasCost)
	if gotDelta.Cmp(wantDelta) != 0 {
		h.t.Fatalf("balance delta plus gas for %s got %s want %s", address.Hex(), gotDelta, wantDelta)
	}
}

func (h *chainHarness) waitOrderStatus(orderID *big.Int, want uint8, timeout time.Duration) {
	h.t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if h.getOrder(orderID).Status == want {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	h.t.Fatalf("order %s did not reach status %d; last=%d", orderID, want, h.getOrder(orderID).Status)
}

func (h *chainHarness) increaseTime(delta *big.Int) {
	h.t.Helper()
	var result any
	if err := rpcCall(h.rpcURL, "evm_increaseTime", []any{delta.Int64()}, &result); err != nil {
		h.t.Fatal(err)
	}
	if err := rpcCall(h.rpcURL, "evm_mine", []any{}, &result); err != nil {
		h.t.Fatal(err)
	}
}

func mustAccount(t *testing.T, privateKeyHex string) account {
	t.Helper()
	key, err := gethcrypto.HexToECDSA(privateKeyHex)
	if err != nil {
		t.Fatal(err)
	}
	return account{key: key, address: gethcrypto.PubkeyToAddress(key.PublicKey)}
}

func hash32(value string) [32]byte {
	return gethcrypto.Keccak256Hash([]byte(value))
}

func hexHash(value [32]byte) string {
	return "0x" + hex.EncodeToString(value[:])
}

func signPersonal(t *testing.T, key *ecdsa.PrivateKey, message string) string {
	t.Helper()
	prefix := "\x19Ethereum Signed Message:\n" + strconv.Itoa(len(message)) + message
	hash := gethcrypto.Keccak256Hash([]byte(prefix))
	signature, err := gethcrypto.Sign(hash.Bytes(), key)
	if err != nil {
		t.Fatal(err)
	}
	signature[64] += 27
	return "0x" + common.Bytes2Hex(signature)
}

func orderRequestMessage(marketAddress common.Address, orderID *big.Int, requestHash string) string {
	return fmt.Sprintf("Agent Nexus order request\nmarketAddress: %s\norderId: %s\nrequestHash: %s", marketAddress.Hex(), orderID, strings.ToLower(requestHash))
}

func deliveryRequestMessage(marketAddress common.Address, orderID *big.Int) string {
	return fmt.Sprintf("Agent Nexus delivery request\nmarketAddress: %s\norderId: %s", marketAddress.Hex(), orderID)
}

func disputeEvidenceMessage(marketAddress common.Address, orderID *big.Int, requestHash string, deliveryHash string, disputeHash string) string {
	return fmt.Sprintf("Agent Nexus dispute evidence\nmarketAddress: %s\norderId: %s\nrequestHash: %s\ndeliveryHash: %s\ndisputeHash: %s", marketAddress.Hex(), orderID, strings.ToLower(requestHash), strings.ToLower(deliveryHash), strings.ToLower(disputeHash))
}

func writeSellerLLMScript(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "seller-llm.sh")
	content := "#!/usr/bin/env bash\ncat >/dev/null\nprintf '%s\\n' '{\"answer\":\"integration delivery\",\"evidence\":\"integration evidence\"}'\n"
	if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

func writeValidatorLLMScript(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "validator-llm.sh")
	content := "#!/usr/bin/env bash\nset -euo pipefail\ninput=$(cat)\ncase \"$input\" in *'\"orderId\"'* ) ;; *) echo 'missing orderId' >&2; exit 2 ;; esac\nif [ \"${VALIDATOR_LLM_API_KEY:-}\" != \"integration-test-key\" ]; then echo 'unexpected api key' >&2; exit 3; fi\nprintf '%s\\n' '{\"releaseToSeller\":true,\"summary\":\"seller wins\",\"reasoning\":\"delivery satisfies the request\",\"buyerClaim\":\"delivery should pass\",\"sellerDeliveryAssessment\":\"sufficient\",\"confidence\":\"high\"}'\n"
	if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

func waitRPC(t *testing.T, rpcURL string) *ethclient.Client {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	var lastErr error
	for time.Now().Before(deadline) {
		client, err := ethclient.Dial(rpcURL)
		if err == nil {
			if _, err := client.ChainID(context.Background()); err == nil {
				return client
			}
			client.Close()
		}
		lastErr = err
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatalf("RPC did not become ready: %v", lastErr)
	return nil
}

func waitReceipt(ctx context.Context, client *ethclient.Client, txHash common.Hash, timeout time.Duration) (*types.Receipt, error) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		receipt, err := client.TransactionReceipt(ctx, txHash)
		if err == nil {
			return receipt, nil
		}
		if !errors.Is(err, ethereum.NotFound) {
			return nil, err
		}
		time.Sleep(100 * time.Millisecond)
	}
	return nil, fmt.Errorf("timeout waiting for receipt %s", txHash.Hex())
}

func freePort(t *testing.T) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	return strconv.Itoa(listener.Addr().(*net.TCPAddr).Port)
}

type processHandle struct {
	cmd    *exec.Cmd
	output *bytes.Buffer
}

func startProcess(t *testing.T, dir string, args []string, env []string) *processHandle {
	t.Helper()
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Dir = dir
	cmd.Env = env
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	output := &bytes.Buffer{}
	cmd.Stdout = output
	cmd.Stderr = output
	if err := cmd.Start(); err != nil {
		t.Fatalf("start %s: %v", strings.Join(args, " "), err)
	}
	return &processHandle{cmd: cmd, output: output}
}

func (p *processHandle) stop() {
	if p == nil || p.cmd == nil || p.cmd.Process == nil {
		return
	}
	stopCmd(p.cmd, 2*time.Second)
}

func stopCmd(cmd *exec.Cmd, timeout time.Duration) {
	if cmd == nil || cmd.Process == nil {
		return
	}
	if pgid, err := syscall.Getpgid(cmd.Process.Pid); err == nil {
		_ = syscall.Kill(-pgid, syscall.SIGTERM)
	} else {
		_ = cmd.Process.Signal(syscall.SIGTERM)
	}
	done := make(chan struct{})
	go func() {
		_ = cmd.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(timeout):
		if pgid, err := syscall.Getpgid(cmd.Process.Pid); err == nil {
			_ = syscall.Kill(-pgid, syscall.SIGKILL)
		} else {
			_ = cmd.Process.Kill()
		}
		<-done
	}
}

func waitHTTP(t *testing.T, url string) {
	t.Helper()
	deadline := time.Now().Add(20 * time.Second)
	var lastBody []byte
	for time.Now().Before(deadline) {
		resp, err := http.Get(url)
		if err == nil {
			body, _ := io.ReadAll(resp.Body)
			_ = resp.Body.Close()
			lastBody = body
			if resp.StatusCode == http.StatusOK {
				return
			}
		}
		time.Sleep(200 * time.Millisecond)
	}
	t.Fatalf("HTTP endpoint not ready: %s last=%s", url, string(lastBody))
}

func postJSON(t *testing.T, url string, payload any, wantStatus int) []byte {
	t.Helper()
	body := postJSONRaw(t, url, payload, wantStatus)
	return body
}

func postJSONRaw(t *testing.T, url string, payload any, wantStatus int) []byte {
	t.Helper()
	body, status := postJSONWithStatus(t, url, payload)
	if status != wantStatus {
		t.Fatalf("POST %s status=%d want=%d body=%s", url, status, wantStatus, string(body))
	}
	return body
}

func postJSONWithStatus(t *testing.T, url string, payload any) ([]byte, int) {
	t.Helper()
	encoded, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := http.Post(url, "application/json", bytes.NewReader(encoded))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	return body, resp.StatusCode
}

func rpcCall(rpcURL string, method string, params []any, out any) error {
	payload := map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  method,
		"params":  params,
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	resp, err := http.Post(rpcURL, "application/json", bytes.NewReader(encoded))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	var decoded struct {
		Result json.RawMessage `json:"result"`
		Error  *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		return err
	}
	if decoded.Error != nil {
		return fmt.Errorf("rpc %s: %s", method, decoded.Error.Message)
	}
	if out != nil {
		return json.Unmarshal(decoded.Result, out)
	}
	return nil
}

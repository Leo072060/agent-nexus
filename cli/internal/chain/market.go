package chain

import (
	"context"
	"crypto/ecdsa"
	"encoding/hex"
	"fmt"
	"math/big"
	"reflect"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	gethcrypto "github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
)

const marketABIJSON = `[
  {
    "inputs": [],
    "name": "getSellers",
    "outputs": [
      {"internalType": "address[]", "name": "", "type": "address[]"}
    ],
    "stateMutability": "view",
    "type": "function"
  },
  {
    "inputs": [
      {"internalType": "address", "name": "seller", "type": "address"}
    ],
    "name": "getSeller",
    "outputs": [
      {"internalType": "bool", "name": "registered", "type": "bool"},
      {"internalType": "bool", "name": "active", "type": "bool"},
      {"internalType": "string", "name": "sellerURI", "type": "string"},
      {"internalType": "uint256", "name": "price", "type": "uint256"},
      {"internalType": "string", "name": "contentURI", "type": "string"},
      {"internalType": "bytes32", "name": "contentHash", "type": "bytes32"},
      {"internalType": "uint256", "name": "deliveryTimeout", "type": "uint256"}
    ],
    "stateMutability": "view",
    "type": "function"
  },
  {
    "inputs": [{"internalType": "address", "name": "validator", "type": "address"}],
    "name": "getValidator",
    "outputs": [
      {"internalType": "bool", "name": "registered", "type": "bool"},
      {"internalType": "bool", "name": "active", "type": "bool"},
      {"internalType": "string", "name": "validatorURI", "type": "string"},
      {"internalType": "uint256", "name": "fee", "type": "uint256"},
      {"internalType": "uint256", "name": "responseTimeout", "type": "uint256"}
    ],
    "stateMutability": "view",
    "type": "function"
  },
  {
    "inputs": [],
    "name": "getOrderCount",
    "outputs": [{"internalType": "uint256", "name": "", "type": "uint256"}],
    "stateMutability": "view",
    "type": "function"
  },
  {
    "inputs": [{"internalType": "uint256", "name": "orderId", "type": "uint256"}],
    "name": "getOrder",
    "outputs": [
      {
        "components": [
          {"internalType": "address", "name": "buyer", "type": "address"},
          {"internalType": "address", "name": "seller", "type": "address"},
          {"internalType": "address", "name": "validator", "type": "address"},
          {"internalType": "uint256", "name": "amount", "type": "uint256"},
          {"internalType": "uint256", "name": "validatorFee", "type": "uint256"},
          {"internalType": "uint256", "name": "validatorBond", "type": "uint256"},
          {"internalType": "bytes32", "name": "listingHash", "type": "bytes32"},
          {"internalType": "bytes32", "name": "requestHash", "type": "bytes32"},
          {"internalType": "bytes32", "name": "deliveryHash", "type": "bytes32"},
          {"internalType": "bytes32", "name": "resolutionHash", "type": "bytes32"},
          {"internalType": "uint256", "name": "createdAt", "type": "uint256"},
          {"internalType": "uint256", "name": "approvalDeadline", "type": "uint256"},
          {"internalType": "uint256", "name": "deliveryDeadline", "type": "uint256"},
          {"internalType": "uint256", "name": "responseDeadline", "type": "uint256"},
          {"internalType": "enum MarketStorage.OrderStatus", "name": "status", "type": "uint8"}
        ],
        "internalType": "struct MarketStorage.Order",
        "name": "",
        "type": "tuple"
      }
    ],
    "stateMutability": "view",
    "type": "function"
  },
  {
    "inputs": [
      {"internalType": "address", "name": "seller", "type": "address"},
      {"internalType": "address", "name": "validator", "type": "address"},
      {"internalType": "bytes32", "name": "requestHash", "type": "bytes32"},
      {"internalType": "uint256", "name": "approvalTimeout", "type": "uint256"}
    ],
    "name": "createOrder",
    "outputs": [{"internalType": "uint256", "name": "orderId", "type": "uint256"}],
    "stateMutability": "payable",
    "type": "function"
  },
  {
    "inputs": [{"internalType": "uint256", "name": "orderId", "type": "uint256"}],
    "name": "openDispute",
    "outputs": [],
    "stateMutability": "nonpayable",
    "type": "function"
  }
]`

const OrderStatusDeliveryCommitted uint8 = 4
const OrderStatusDisputed uint8 = 5

type MarketClient struct {
	client        *ethclient.Client
	marketAddress common.Address
	abi           abi.ABI
	privateKey    *ecdsa.PrivateKey
}

type Seller struct {
	Address         common.Address
	Registered      bool
	Active          bool
	SellerURI       string
	Price           *big.Int
	ContentURI      string
	ContentHash     string
	DeliveryTimeout *big.Int
}

type Validator struct {
	Address         common.Address
	Registered      bool
	Active          bool
	ValidatorURI    string
	Fee             *big.Int
	ResponseTimeout *big.Int
}

type CreateOrderResult struct {
	OrderID *big.Int
	TxHash  string
}

type Order struct {
	Buyer        common.Address
	Seller       common.Address
	Validator    common.Address
	DeliveryHash string
	RequestHash  string
	Status       uint8
}

func (m *MarketClient) Address() common.Address {
	return m.marketAddress
}

func NewMarketClient(ctx context.Context, rpcURL string, marketAddress string) (*MarketClient, error) {
	return NewMarketClientWithPrivateKey(ctx, rpcURL, marketAddress, nil)
}

func NewMarketClientWithPrivateKey(ctx context.Context, rpcURL string, marketAddress string, privateKey *ecdsa.PrivateKey) (*MarketClient, error) {
	if !common.IsHexAddress(marketAddress) {
		return nil, fmt.Errorf("invalid market address")
	}

	parsedABI, err := abi.JSON(strings.NewReader(marketABIJSON))
	if err != nil {
		return nil, fmt.Errorf("parse market ABI: %w", err)
	}

	client, err := ethclient.DialContext(ctx, rpcURL)
	if err != nil {
		return nil, fmt.Errorf("connect RPC: %w", err)
	}

	return &MarketClient{
		client:        client,
		marketAddress: common.HexToAddress(marketAddress),
		abi:           parsedABI,
		privateKey:    privateKey,
	}, nil
}

func (m *MarketClient) Close() {
	m.client.Close()
}

func (m *MarketClient) GetSellers(ctx context.Context) ([]common.Address, error) {
	outputs, err := m.call(ctx, "getSellers")
	if err != nil {
		return nil, err
	}

	sellers, ok := outputs[0].([]common.Address)
	if !ok {
		return nil, fmt.Errorf("decode getSellers output")
	}

	return sellers, nil
}

func (m *MarketClient) GetSeller(ctx context.Context, seller common.Address) (Seller, error) {
	outputs, err := m.call(ctx, "getSeller", seller)
	if err != nil {
		return Seller{}, err
	}

	contentHash, ok := outputs[5].([32]byte)
	if !ok {
		return Seller{}, fmt.Errorf("decode contentHash")
	}

	return Seller{
		Address:         seller,
		Registered:      outputs[0].(bool),
		Active:          outputs[1].(bool),
		SellerURI:       outputs[2].(string),
		Price:           outputs[3].(*big.Int),
		ContentURI:      outputs[4].(string),
		ContentHash:     "0x" + hex.EncodeToString(contentHash[:]),
		DeliveryTimeout: outputs[6].(*big.Int),
	}, nil
}

func (m *MarketClient) GetValidator(ctx context.Context, validator common.Address) (Validator, error) {
	outputs, err := m.call(ctx, "getValidator", validator)
	if err != nil {
		return Validator{}, err
	}

	return Validator{
		Address:         validator,
		Registered:      outputs[0].(bool),
		Active:          outputs[1].(bool),
		ValidatorURI:    outputs[2].(string),
		Fee:             outputs[3].(*big.Int),
		ResponseTimeout: outputs[4].(*big.Int),
	}, nil
}

func (m *MarketClient) GetOrderCount(ctx context.Context) (*big.Int, error) {
	outputs, err := m.call(ctx, "getOrderCount")
	if err != nil {
		return nil, err
	}

	count, ok := outputs[0].(*big.Int)
	if !ok {
		return nil, fmt.Errorf("decode getOrderCount output")
	}

	return count, nil
}

func (m *MarketClient) GetOrder(ctx context.Context, orderID *big.Int) (Order, error) {
	outputs, err := m.call(ctx, "getOrder", orderID)
	if err != nil {
		return Order{}, err
	}
	if len(outputs) != 1 {
		return Order{}, fmt.Errorf("decode getOrder output")
	}

	value := reflect.ValueOf(outputs[0])
	if value.Kind() != reflect.Struct {
		return Order{}, fmt.Errorf("decode getOrder tuple")
	}

	return Order{
		Buyer:        value.FieldByName("Buyer").Interface().(common.Address),
		Seller:       value.FieldByName("Seller").Interface().(common.Address),
		Validator:    value.FieldByName("Validator").Interface().(common.Address),
		RequestHash:  bytes32Hex(value.FieldByName("RequestHash").Interface().([32]byte)),
		DeliveryHash: bytes32Hex(value.FieldByName("DeliveryHash").Interface().([32]byte)),
		Status:       value.FieldByName("Status").Interface().(uint8),
	}, nil
}

func (m *MarketClient) CreateOrder(ctx context.Context, seller common.Address, validator common.Address, requestHash [32]byte, approvalTimeout *big.Int, value *big.Int) (CreateOrderResult, error) {
	before, err := m.GetOrderCount(ctx)
	if err != nil {
		return CreateOrderResult{}, err
	}

	input, err := m.abi.Pack("createOrder", seller, validator, requestHash, approvalTimeout)
	if err != nil {
		return CreateOrderResult{}, fmt.Errorf("pack createOrder: %w", err)
	}

	txHash, err := m.sendTransactionWithValue(ctx, input, value)
	if err != nil {
		return CreateOrderResult{}, err
	}
	if err := m.waitTransactionMined(ctx, txHash, 30*time.Second); err != nil {
		return CreateOrderResult{}, err
	}

	after, err := m.GetOrderCount(ctx)
	if err != nil {
		return CreateOrderResult{}, err
	}
	want := new(big.Int).Add(before, big.NewInt(1))
	if after.Cmp(want) != 0 {
		return CreateOrderResult{}, fmt.Errorf("order count mismatch after createOrder: got %s want %s", after.String(), want.String())
	}

	return CreateOrderResult{OrderID: after, TxHash: txHash}, nil
}

func (m *MarketClient) OpenDispute(ctx context.Context, orderID *big.Int) (string, error) {
	input, err := m.abi.Pack("openDispute", orderID)
	if err != nil {
		return "", fmt.Errorf("pack openDispute: %w", err)
	}

	return m.sendTransaction(ctx, input)
}

func (m *MarketClient) call(ctx context.Context, method string, args ...any) ([]any, error) {
	input, err := m.abi.Pack(method, args...)
	if err != nil {
		return nil, fmt.Errorf("pack %s: %w", method, err)
	}

	result, err := m.client.CallContract(ctx, ethereum.CallMsg{
		To:   &m.marketAddress,
		Data: input,
	}, nil)
	if err != nil {
		return nil, fmt.Errorf("call %s: %w", method, err)
	}
	if len(result) == 0 {
		return nil, fmt.Errorf("call %s: empty response from market contract", method)
	}

	outputs, err := m.abi.Unpack(method, result)
	if err != nil {
		return nil, fmt.Errorf("unpack %s: %w", method, err)
	}

	return outputs, nil
}

func (m *MarketClient) sendTransaction(ctx context.Context, input []byte) (string, error) {
	return m.sendTransactionWithValue(ctx, input, big.NewInt(0))
}

func (m *MarketClient) sendTransactionWithValue(ctx context.Context, input []byte, value *big.Int) (string, error) {
	if m.privateKey == nil {
		return "", fmt.Errorf("private key is required for transaction")
	}
	if value == nil {
		value = big.NewInt(0)
	}

	from := gethcrypto.PubkeyToAddress(m.privateKey.PublicKey)
	chainID, err := m.client.ChainID(ctx)
	if err != nil {
		return "", fmt.Errorf("read chain id: %w", err)
	}
	nonce, err := m.client.PendingNonceAt(ctx, from)
	if err != nil {
		return "", fmt.Errorf("read nonce: %w", err)
	}
	gasPrice, err := m.client.SuggestGasPrice(ctx)
	if err != nil {
		return "", fmt.Errorf("suggest gas price: %w", err)
	}
	gasLimit, err := m.client.EstimateGas(ctx, ethereum.CallMsg{
		From:  from,
		To:    &m.marketAddress,
		Value: value,
		Data:  input,
	})
	if err != nil {
		return "", fmt.Errorf("estimate gas: %w", err)
	}

	tx := types.NewTransaction(nonce, m.marketAddress, value, gasLimit, gasPrice, input)
	signedTx, err := types.SignTx(tx, types.LatestSignerForChainID(chainID), m.privateKey)
	if err != nil {
		return "", fmt.Errorf("sign transaction: %w", err)
	}
	if err := m.client.SendTransaction(ctx, signedTx); err != nil {
		return "", fmt.Errorf("send transaction: %w", err)
	}

	return signedTx.Hash().Hex(), nil
}

func (m *MarketClient) waitTransactionMined(ctx context.Context, txHash string, timeout time.Duration) error {
	hash := common.HexToHash(txHash)
	deadline := time.Now().Add(timeout)
	for {
		receipt, err := m.client.TransactionReceipt(ctx, hash)
		if err == nil {
			if receipt.Status != types.ReceiptStatusSuccessful {
				return fmt.Errorf("transaction reverted: %s", txHash)
			}
			return nil
		}
		if err != ethereum.NotFound {
			return fmt.Errorf("read transaction receipt: %w", err)
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("timed out waiting for transaction: %s", txHash)
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(500 * time.Millisecond):
		}
	}
}

func bytes32Hex(value [32]byte) string {
	return "0x" + hex.EncodeToString(value[:])
}

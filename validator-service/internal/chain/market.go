package chain

import (
	"context"
	"crypto/ecdsa"
	"encoding/hex"
	"fmt"
	"math/big"
	"reflect"
	"strings"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	gethcrypto "github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
)

const marketABIJSON = `[
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
      {"internalType": "uint256", "name": "orderId", "type": "uint256"},
      {"internalType": "bool", "name": "releaseToSeller", "type": "bool"},
      {"internalType": "bytes32", "name": "resolutionHash", "type": "bytes32"}
    ],
    "name": "resolveDispute",
    "outputs": [],
    "stateMutability": "nonpayable",
    "type": "function"
  }
]`

const OrderStatusDisputed uint8 = 5

type MarketClient struct {
	client        *ethclient.Client
	marketAddress common.Address
	abi           abi.ABI
	privateKey    *ecdsa.PrivateKey
}

type Order struct {
	Buyer        common.Address
	Seller       common.Address
	Validator    common.Address
	RequestHash  string
	DeliveryHash string
	Status       uint8
}

func NewMarketClient(ctx context.Context, rpcURL string, marketAddress common.Address, privateKey *ecdsa.PrivateKey) (*MarketClient, error) {
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
		marketAddress: marketAddress,
		abi:           parsedABI,
		privateKey:    privateKey,
	}, nil
}

func (m *MarketClient) Close() {
	m.client.Close()
}

func (m *MarketClient) GetOrder(ctx context.Context, orderID *big.Int) (Order, error) {
	input, err := m.abi.Pack("getOrder", orderID)
	if err != nil {
		return Order{}, fmt.Errorf("pack getOrder: %w", err)
	}

	result, err := m.client.CallContract(ctx, ethereum.CallMsg{
		To:   &m.marketAddress,
		Data: input,
	}, nil)
	if err != nil {
		return Order{}, fmt.Errorf("call getOrder: %w", err)
	}

	outputs, err := m.abi.Unpack("getOrder", result)
	if err != nil {
		return Order{}, fmt.Errorf("unpack getOrder: %w", err)
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

func (m *MarketClient) ResolveDispute(ctx context.Context, orderID *big.Int, releaseToSeller bool, resolutionHash [32]byte) (string, error) {
	input, err := m.abi.Pack("resolveDispute", orderID, releaseToSeller, resolutionHash)
	if err != nil {
		return "", fmt.Errorf("pack resolveDispute: %w", err)
	}

	return m.sendTransaction(ctx, input)
}

func (m *MarketClient) sendTransaction(ctx context.Context, input []byte) (string, error) {
	if m.privateKey == nil {
		return "", fmt.Errorf("private key is required for transaction")
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
		From: from,
		To:   &m.marketAddress,
		Data: input,
	})
	if err != nil {
		return "", fmt.Errorf("estimate gas: %w", err)
	}

	tx := types.NewTransaction(nonce, m.marketAddress, big.NewInt(0), gasLimit, gasPrice, input)
	signedTx, err := types.SignTx(tx, types.LatestSignerForChainID(chainID), m.privateKey)
	if err != nil {
		return "", fmt.Errorf("sign transaction: %w", err)
	}
	if err := m.client.SendTransaction(ctx, signedTx); err != nil {
		return "", fmt.Errorf("send transaction: %w", err)
	}

	return signedTx.Hash().Hex(), nil
}

func bytes32Hex(value [32]byte) string {
	return "0x" + hex.EncodeToString(value[:])
}

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
    "inputs": [{"internalType": "uint256", "name": "orderId", "type": "uint256"}],
    "name": "confirmAsSeller",
    "outputs": [],
    "stateMutability": "nonpayable",
    "type": "function"
  },
  {
    "inputs": [
      {"internalType": "uint256", "name": "orderId", "type": "uint256"},
      {"internalType": "bytes32", "name": "deliveryHash", "type": "bytes32"}
    ],
    "name": "commitDelivery",
    "outputs": [],
    "stateMutability": "nonpayable",
    "type": "function"
  }
]`

const (
	OrderStatusDeliveryCommitted uint8 = 4
)

type MarketClient struct {
	client        *ethclient.Client
	marketAddress common.Address
	abi           abi.ABI
	privateKey    *ecdsa.PrivateKey
}

type Order struct {
	Buyer            common.Address
	Seller           common.Address
	Validator        common.Address
	Amount           *big.Int
	ValidatorFee     *big.Int
	ListingHash      string
	RequestHash      string
	DeliveryHash     string
	ResolutionHash   string
	CreatedAt        *big.Int
	ApprovalDeadline *big.Int
	DeliveryDeadline *big.Int
	ResponseDeadline *big.Int
	Status           uint8
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
	outputs, err := m.call(ctx, "getOrder", orderID)
	if err != nil {
		return Order{}, err
	}
	if len(outputs) != 1 {
		return Order{}, fmt.Errorf("decode getOrder output")
	}

	return decodeOrder(outputs[0])
}

func (m *MarketClient) PackConfirmAsSeller(orderID *big.Int) ([]byte, error) {
	return m.abi.Pack("confirmAsSeller", orderID)
}

func (m *MarketClient) PackCommitDelivery(orderID *big.Int, deliveryHash [32]byte) ([]byte, error) {
	return m.abi.Pack("commitDelivery", orderID, deliveryHash)
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

func decodeOrder(value any) (Order, error) {
	v := reflect.ValueOf(value)
	if v.Kind() != reflect.Struct {
		return Order{}, fmt.Errorf("getOrder returned %T, expected tuple struct", value)
	}

	order := Order{
		Buyer:            field[common.Address](v, "Buyer"),
		Seller:           field[common.Address](v, "Seller"),
		Validator:        field[common.Address](v, "Validator"),
		Amount:           field[*big.Int](v, "Amount"),
		ValidatorFee:     field[*big.Int](v, "ValidatorFee"),
		ListingHash:      bytes32Hex(field[[32]byte](v, "ListingHash")),
		RequestHash:      bytes32Hex(field[[32]byte](v, "RequestHash")),
		DeliveryHash:     bytes32Hex(field[[32]byte](v, "DeliveryHash")),
		ResolutionHash:   bytes32Hex(field[[32]byte](v, "ResolutionHash")),
		CreatedAt:        field[*big.Int](v, "CreatedAt"),
		ApprovalDeadline: field[*big.Int](v, "ApprovalDeadline"),
		DeliveryDeadline: field[*big.Int](v, "DeliveryDeadline"),
		ResponseDeadline: field[*big.Int](v, "ResponseDeadline"),
		Status:           field[uint8](v, "Status"),
	}

	return order, nil
}

func field[T any](v reflect.Value, name string) T {
	return v.FieldByName(name).Interface().(T)
}

func bytes32Hex(value [32]byte) string {
	return "0x" + hex.EncodeToString(value[:])
}

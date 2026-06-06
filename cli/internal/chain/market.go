package chain

import (
	"context"
	"encoding/hex"
	"fmt"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
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
  }
]`

type MarketClient struct {
	client        *ethclient.Client
	marketAddress common.Address
	abi           abi.ABI
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

func NewMarketClient(ctx context.Context, rpcURL string, marketAddress string) (*MarketClient, error) {
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

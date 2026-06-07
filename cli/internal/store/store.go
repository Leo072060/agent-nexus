package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

type Store struct {
	db *sql.DB
}

type Request struct {
	ID          int64  `json:"id"`
	RequestHash string `json:"requestHash"`
	Content     string `json:"content,omitempty"`
	ContentPath string `json:"contentPath,omitempty"`
	CreatedAt   string `json:"createdAt"`
}

type Market struct {
	ID            int64  `json:"id"`
	Name          string `json:"name"`
	RPCURL        string `json:"rpcUrl"`
	MarketAddress string `json:"marketAddress"`
	Active        bool   `json:"active"`
	CreatedAt     string `json:"createdAt"`
	UpdatedAt     string `json:"updatedAt"`
}

type Order struct {
	ID                int64    `json:"id"`
	RPCURL            string   `json:"rpcUrl"`
	MarketAddress     string   `json:"marketAddress"`
	ChainOrderID      *big.Int `json:"chainOrderId"`
	BuyerAddress      string   `json:"buyerAddress,omitempty"`
	SellerURI         string   `json:"sellerUri"`
	ValidatorURI      string   `json:"validatorUri,omitempty"`
	RequestContent    string   `json:"requestContent,omitempty"`
	DeliveryHash      string   `json:"deliveryHash,omitempty"`
	Delivery          string   `json:"delivery,omitempty"`
	Dispute           string   `json:"dispute,omitempty"`
	OpenDisputeTxHash string   `json:"openDisputeTxHash,omitempty"`
	Status            string   `json:"status"`
	CreatedAt         string   `json:"createdAt"`
	UpdatedAt         string   `json:"updatedAt"`
}

type CreateRequestInput struct {
	RequestHash string
	Content     string
	ContentPath string
}

type CreateMarketInput struct {
	Name          string
	RPCURL        string
	MarketAddress string
}

type CreateOrderInput struct {
	RPCURL         string
	MarketAddress  string
	ChainOrderID   *big.Int
	BuyerAddress   string
	SellerURI      string
	ValidatorURI   string
	RequestContent string
	Status         string
}

func Open(path string) (*Store, error) {
	if path == "" {
		return nil, errors.New("database path is required")
	}

	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	store := &Store{db: db}
	if err := store.migrate(context.Background()); err != nil {
		db.Close()
		return nil, err
	}

	return store, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) migrate(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS requests (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	request_hash TEXT NOT NULL,
	content TEXT,
	content_path TEXT,
	created_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS markets (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	name TEXT NOT NULL UNIQUE,
	rpc_url TEXT NOT NULL,
	market_address TEXT NOT NULL,
	active INTEGER NOT NULL DEFAULT 0,
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS orders (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	rpc_url TEXT NOT NULL,
	market_address TEXT NOT NULL,
	chain_order_id TEXT NOT NULL,
	buyer_address TEXT NOT NULL DEFAULT '',
	seller_uri TEXT NOT NULL,
	validator_uri TEXT NOT NULL DEFAULT '',
	request_content TEXT NOT NULL DEFAULT '',
	delivery_hash TEXT,
	delivery TEXT,
	dispute TEXT NOT NULL DEFAULT '',
	open_dispute_tx_hash TEXT NOT NULL DEFAULT '',
	status TEXT NOT NULL,
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL
);
`)
	if err != nil {
		return fmt.Errorf("migrate database: %w", err)
	}

	for _, stmt := range []string{
		`ALTER TABLE orders ADD COLUMN buyer_address TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE orders ADD COLUMN validator_uri TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE orders ADD COLUMN request_content TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE orders ADD COLUMN dispute TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE orders ADD COLUMN open_dispute_tx_hash TEXT NOT NULL DEFAULT ''`,
	} {
		if _, err := s.db.ExecContext(ctx, stmt); err != nil && !strings.Contains(err.Error(), "duplicate column name") {
			return fmt.Errorf("migrate database: %w", err)
		}
	}

	return nil
}

func (s *Store) CreateOrder(ctx context.Context, input CreateOrderInput) (Order, error) {
	if input.ChainOrderID == nil || input.ChainOrderID.Sign() <= 0 {
		return Order{}, fmt.Errorf("chain order id is required")
	}

	now := time.Now().UTC().Format(time.RFC3339)
	result, err := s.db.ExecContext(
		ctx,
		`INSERT INTO orders (
	rpc_url,
	market_address,
	chain_order_id,
	buyer_address,
	seller_uri,
	validator_uri,
	request_content,
	status,
	created_at,
	updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		input.RPCURL,
		input.MarketAddress,
		input.ChainOrderID.String(),
		input.BuyerAddress,
		input.SellerURI,
		input.ValidatorURI,
		input.RequestContent,
		input.Status,
		now,
		now,
	)
	if err != nil {
		return Order{}, fmt.Errorf("insert order: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return Order{}, fmt.Errorf("get order id: %w", err)
	}

	return Order{
		ID:             id,
		RPCURL:         input.RPCURL,
		MarketAddress:  input.MarketAddress,
		ChainOrderID:   new(big.Int).Set(input.ChainOrderID),
		BuyerAddress:   input.BuyerAddress,
		SellerURI:      input.SellerURI,
		ValidatorURI:   input.ValidatorURI,
		RequestContent: input.RequestContent,
		Status:         input.Status,
		CreatedAt:      now,
		UpdatedAt:      now,
	}, nil
}

func (s *Store) UpdateOrderStatus(ctx context.Context, id int64, status string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	result, err := s.db.ExecContext(
		ctx,
		`UPDATE orders SET status = ?, updated_at = ? WHERE id = ?`,
		status,
		now,
		id,
	)
	if err != nil {
		return fmt.Errorf("update order status: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("check updated order: %w", err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("order not found: %d", id)
	}

	return nil
}

func (s *Store) GetOrder(ctx context.Context, id int64) (Order, error) {
	var order Order
	var chainOrderID string

	err := s.db.QueryRowContext(
		ctx,
		`SELECT
	id,
	rpc_url,
	market_address,
	chain_order_id,
	COALESCE(buyer_address, ''),
	seller_uri,
	COALESCE(validator_uri, ''),
	COALESCE(request_content, ''),
	COALESCE(delivery_hash, ''),
	COALESCE(delivery, ''),
	COALESCE(dispute, ''),
	COALESCE(open_dispute_tx_hash, ''),
	status,
	created_at,
	updated_at
FROM orders WHERE id = ?`,
		id,
	).Scan(
		&order.ID,
		&order.RPCURL,
		&order.MarketAddress,
		&chainOrderID,
		&order.BuyerAddress,
		&order.SellerURI,
		&order.ValidatorURI,
		&order.RequestContent,
		&order.DeliveryHash,
		&order.Delivery,
		&order.Dispute,
		&order.OpenDisputeTxHash,
		&order.Status,
		&order.CreatedAt,
		&order.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return Order{}, fmt.Errorf("order not found: %d", id)
	}
	if err != nil {
		return Order{}, fmt.Errorf("get order: %w", err)
	}

	parsedOrderID, ok := new(big.Int).SetString(chainOrderID, 10)
	if !ok {
		return Order{}, fmt.Errorf("invalid chain_order_id for order %d", id)
	}
	order.ChainOrderID = parsedOrderID

	return order, nil
}

func (s *Store) UpdateOrderDelivery(ctx context.Context, id int64, deliveryHash string, delivery string, status string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	result, err := s.db.ExecContext(
		ctx,
		`UPDATE orders SET delivery_hash = ?, delivery = ?, status = ?, updated_at = ? WHERE id = ?`,
		deliveryHash,
		delivery,
		status,
		now,
		id,
	)
	if err != nil {
		return fmt.Errorf("update order delivery: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("check updated order: %w", err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("order not found: %d", id)
	}

	return nil
}

func (s *Store) UpdateOrderDispute(ctx context.Context, id int64, requestContent string, delivery string, dispute string, txHash string, status string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	result, err := s.db.ExecContext(
		ctx,
		`UPDATE orders SET request_content = ?, delivery = ?, dispute = ?, open_dispute_tx_hash = ?, status = ?, updated_at = ? WHERE id = ?`,
		requestContent,
		delivery,
		dispute,
		txHash,
		status,
		now,
		id,
	)
	if err != nil {
		return fmt.Errorf("update order dispute: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("check updated order: %w", err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("order not found: %d", id)
	}

	return nil
}

func (s *Store) CreateRequest(ctx context.Context, input CreateRequestInput) (Request, error) {
	createdAt := time.Now().UTC().Format(time.RFC3339)

	result, err := s.db.ExecContext(
		ctx,
		`INSERT INTO requests (request_hash, content, content_path, created_at) VALUES (?, ?, ?, ?)`,
		input.RequestHash,
		input.Content,
		input.ContentPath,
		createdAt,
	)
	if err != nil {
		return Request{}, fmt.Errorf("insert request: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return Request{}, fmt.Errorf("get request id: %w", err)
	}

	return Request{
		ID:          id,
		RequestHash: input.RequestHash,
		Content:     input.Content,
		ContentPath: input.ContentPath,
		CreatedAt:   createdAt,
	}, nil
}

func (s *Store) GetRequest(ctx context.Context, id int64) (Request, error) {
	var request Request
	err := s.db.QueryRowContext(
		ctx,
		`SELECT id, request_hash, content, content_path, created_at FROM requests WHERE id = ?`,
		id,
	).Scan(
		&request.ID,
		&request.RequestHash,
		&request.Content,
		&request.ContentPath,
		&request.CreatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return Request{}, fmt.Errorf("request not found: %d", id)
	}
	if err != nil {
		return Request{}, fmt.Errorf("get request: %w", err)
	}

	return request, nil
}

func (s *Store) ListRequests(ctx context.Context) ([]Request, error) {
	rows, err := s.db.QueryContext(
		ctx,
		`SELECT id, request_hash, content, content_path, created_at FROM requests ORDER BY id ASC`,
	)
	if err != nil {
		return nil, fmt.Errorf("list requests: %w", err)
	}
	defer rows.Close()

	var requests []Request
	for rows.Next() {
		var request Request
		if err := rows.Scan(
			&request.ID,
			&request.RequestHash,
			&request.Content,
			&request.ContentPath,
			&request.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan request: %w", err)
		}

		requests = append(requests, request)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate requests: %w", err)
	}

	if requests == nil {
		requests = []Request{}
	}

	return requests, nil
}

func (s *Store) CreateMarket(ctx context.Context, input CreateMarketInput) (Market, error) {
	now := time.Now().UTC().Format(time.RFC3339)

	result, err := s.db.ExecContext(
		ctx,
		`INSERT INTO markets (name, rpc_url, market_address, active, created_at, updated_at) VALUES (?, ?, ?, 0, ?, ?)`,
		input.Name,
		input.RPCURL,
		input.MarketAddress,
		now,
		now,
	)
	if err != nil {
		return Market{}, fmt.Errorf("insert market: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return Market{}, fmt.Errorf("get market id: %w", err)
	}

	return Market{
		ID:            id,
		Name:          input.Name,
		RPCURL:        input.RPCURL,
		MarketAddress: input.MarketAddress,
		Active:        false,
		CreatedAt:     now,
		UpdatedAt:     now,
	}, nil
}

func (s *Store) ActivateMarket(ctx context.Context, name string) (Market, error) {
	now := time.Now().UTC().Format(time.RFC3339)

	result, err := s.db.ExecContext(ctx, `UPDATE markets SET active = 1, updated_at = ? WHERE name = ?`, now, name)
	if err != nil {
		return Market{}, fmt.Errorf("activate market: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return Market{}, fmt.Errorf("check activated market: %w", err)
	}
	if rowsAffected == 0 {
		return Market{}, fmt.Errorf("market not found: %s", name)
	}

	return s.GetMarket(ctx, name)
}

func (s *Store) DeactivateMarket(ctx context.Context, name string) (Market, error) {
	now := time.Now().UTC().Format(time.RFC3339)

	result, err := s.db.ExecContext(ctx, `UPDATE markets SET active = 0, updated_at = ? WHERE name = ?`, now, name)
	if err != nil {
		return Market{}, fmt.Errorf("deactivate market: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return Market{}, fmt.Errorf("check deactivated market: %w", err)
	}
	if rowsAffected == 0 {
		return Market{}, fmt.Errorf("market not found: %s", name)
	}

	return s.GetMarket(ctx, name)
}

func (s *Store) GetMarket(ctx context.Context, name string) (Market, error) {
	market, err := scanMarket(s.db.QueryRowContext(
		ctx,
		`SELECT id, name, rpc_url, market_address, active, created_at, updated_at FROM markets WHERE name = ?`,
		name,
	))
	if errors.Is(err, sql.ErrNoRows) {
		return Market{}, fmt.Errorf("market not found: %s", name)
	}
	if err != nil {
		return Market{}, fmt.Errorf("get market: %w", err)
	}

	return market, nil
}

func (s *Store) ListActiveMarkets(ctx context.Context) ([]Market, error) {
	rows, err := s.db.QueryContext(
		ctx,
		`SELECT id, name, rpc_url, market_address, active, created_at, updated_at FROM markets WHERE active = 1 ORDER BY id ASC`,
	)
	if err != nil {
		return nil, fmt.Errorf("list active markets: %w", err)
	}
	defer rows.Close()

	return scanMarkets(rows)
}

func (s *Store) ListMarkets(ctx context.Context) ([]Market, error) {
	rows, err := s.db.QueryContext(
		ctx,
		`SELECT id, name, rpc_url, market_address, active, created_at, updated_at FROM markets ORDER BY id ASC`,
	)
	if err != nil {
		return nil, fmt.Errorf("list markets: %w", err)
	}
	defer rows.Close()

	return scanMarkets(rows)
}

func scanMarkets(rows *sql.Rows) ([]Market, error) {
	var markets []Market
	for rows.Next() {
		market, err := scanMarket(rows)
		if err != nil {
			return nil, fmt.Errorf("scan market: %w", err)
		}
		markets = append(markets, market)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate markets: %w", err)
	}

	if markets == nil {
		markets = []Market{}
	}

	return markets, nil
}

type marketScanner interface {
	Scan(dest ...any) error
}

func scanMarket(scanner marketScanner) (Market, error) {
	var market Market
	var active int

	err := scanner.Scan(
		&market.ID,
		&market.Name,
		&market.RPCURL,
		&market.MarketAddress,
		&active,
		&market.CreatedAt,
		&market.UpdatedAt,
	)
	if err != nil {
		return Market{}, err
	}

	market.Active = active == 1
	return market, nil
}

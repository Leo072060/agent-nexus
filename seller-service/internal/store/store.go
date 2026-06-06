package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math/big"
	"time"

	_ "modernc.org/sqlite"
)

type Store struct {
	db *sql.DB
}

type Delivery struct {
	ID           int64
	ChainOrderID *big.Int
	DeliveryHash string
	DeliveryBody []byte
	CreatedAt    string
	UpdatedAt    string
}

type Order struct {
	ID                   int64
	ChainOrderID         *big.Int
	BuyerAddress         string
	SellerAddress        string
	ValidatorAddress     string
	RequestHash          string
	RequestBody          []byte
	ConfirmSellerTxHash  string
	DeliveryHash         string
	DeliveryBody         []byte
	CommitDeliveryTxHash string
	Status               string
	CreatedAt            string
	UpdatedAt            string
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
CREATE TABLE IF NOT EXISTS deliveries (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	chain_order_id TEXT NOT NULL UNIQUE,
	delivery_hash TEXT NOT NULL,
	delivery_body BLOB NOT NULL,
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS orders (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	chain_order_id TEXT NOT NULL UNIQUE,
	buyer_address TEXT NOT NULL,
	seller_address TEXT NOT NULL,
	validator_address TEXT NOT NULL,
	request_hash TEXT NOT NULL,
	request_body BLOB NOT NULL,
	confirm_seller_tx_hash TEXT NOT NULL DEFAULT '',
	delivery_hash TEXT NOT NULL DEFAULT '',
	delivery_body BLOB,
	commit_delivery_tx_hash TEXT NOT NULL DEFAULT '',
	status TEXT NOT NULL,
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL
);
`)
	if err != nil {
		return fmt.Errorf("migrate database: %w", err)
	}

	return nil
}

func (s *Store) UpsertDelivery(ctx context.Context, chainOrderID *big.Int, deliveryHash string, body []byte) (Delivery, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	chainOrderIDText := chainOrderID.String()

	_, err := s.db.ExecContext(
		ctx,
		`INSERT INTO deliveries (chain_order_id, delivery_hash, delivery_body, created_at, updated_at)
VALUES (?, ?, ?, ?, ?)
ON CONFLICT(chain_order_id) DO UPDATE SET
	delivery_hash = excluded.delivery_hash,
	delivery_body = excluded.delivery_body,
	updated_at = excluded.updated_at`,
		chainOrderIDText,
		deliveryHash,
		body,
		now,
		now,
	)
	if err != nil {
		return Delivery{}, fmt.Errorf("upsert delivery: %w", err)
	}

	return s.GetDelivery(ctx, chainOrderID)
}

func (s *Store) GetDelivery(ctx context.Context, chainOrderID *big.Int) (Delivery, error) {
	var delivery Delivery
	var chainOrderIDText string
	err := s.db.QueryRowContext(
		ctx,
		`SELECT id, chain_order_id, delivery_hash, delivery_body, created_at, updated_at FROM deliveries WHERE chain_order_id = ?`,
		chainOrderID.String(),
	).Scan(
		&delivery.ID,
		&chainOrderIDText,
		&delivery.DeliveryHash,
		&delivery.DeliveryBody,
		&delivery.CreatedAt,
		&delivery.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return Delivery{}, fmt.Errorf("delivery not found: %s", chainOrderID.String())
	}
	if err != nil {
		return Delivery{}, fmt.Errorf("get delivery: %w", err)
	}

	parsedID, ok := new(big.Int).SetString(chainOrderIDText, 10)
	if !ok {
		return Delivery{}, fmt.Errorf("invalid stored chain_order_id: %s", chainOrderIDText)
	}
	delivery.ChainOrderID = parsedID

	return delivery, nil
}

func (s *Store) UpsertOrderRequest(
	ctx context.Context,
	chainOrderID *big.Int,
	buyerAddress string,
	sellerAddress string,
	validatorAddress string,
	requestHash string,
	requestBody []byte,
	status string,
) (Order, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	chainOrderIDText := chainOrderID.String()

	_, err := s.db.ExecContext(
		ctx,
		`INSERT INTO orders (
	chain_order_id,
	buyer_address,
	seller_address,
	validator_address,
	request_hash,
	request_body,
	status,
	created_at,
	updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(chain_order_id) DO UPDATE SET
	buyer_address = excluded.buyer_address,
	seller_address = excluded.seller_address,
	validator_address = excluded.validator_address,
	request_hash = excluded.request_hash,
	request_body = excluded.request_body,
	status = excluded.status,
	updated_at = excluded.updated_at`,
		chainOrderIDText,
		buyerAddress,
		sellerAddress,
		validatorAddress,
		requestHash,
		requestBody,
		status,
		now,
		now,
	)
	if err != nil {
		return Order{}, fmt.Errorf("upsert order request: %w", err)
	}

	return s.GetOrder(ctx, chainOrderID)
}

func (s *Store) MarkSellerConfirmed(ctx context.Context, chainOrderID *big.Int, txHash string) error {
	return s.updateOrder(ctx, chainOrderID, `confirm_seller_tx_hash = ?, status = ?`, txHash, "seller_confirmed")
}

func (s *Store) MarkDeliveryCommitted(ctx context.Context, chainOrderID *big.Int, deliveryHash string, deliveryBody []byte, txHash string) error {
	if _, err := s.UpsertDelivery(ctx, chainOrderID, deliveryHash, deliveryBody); err != nil {
		return err
	}

	return s.updateOrder(
		ctx,
		chainOrderID,
		`delivery_hash = ?, delivery_body = ?, commit_delivery_tx_hash = ?, status = ?`,
		deliveryHash,
		deliveryBody,
		txHash,
		"delivery_committed",
	)
}

func (s *Store) GetOrder(ctx context.Context, chainOrderID *big.Int) (Order, error) {
	var order Order
	var chainOrderIDText string
	var deliveryBody []byte
	err := s.db.QueryRowContext(
		ctx,
		`SELECT
	id,
	chain_order_id,
	buyer_address,
	seller_address,
	validator_address,
	request_hash,
	request_body,
	confirm_seller_tx_hash,
	delivery_hash,
	delivery_body,
	commit_delivery_tx_hash,
	status,
	created_at,
	updated_at
FROM orders WHERE chain_order_id = ?`,
		chainOrderID.String(),
	).Scan(
		&order.ID,
		&chainOrderIDText,
		&order.BuyerAddress,
		&order.SellerAddress,
		&order.ValidatorAddress,
		&order.RequestHash,
		&order.RequestBody,
		&order.ConfirmSellerTxHash,
		&order.DeliveryHash,
		&deliveryBody,
		&order.CommitDeliveryTxHash,
		&order.Status,
		&order.CreatedAt,
		&order.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return Order{}, fmt.Errorf("order not found: %s", chainOrderID.String())
	}
	if err != nil {
		return Order{}, fmt.Errorf("get order: %w", err)
	}

	parsedID, ok := new(big.Int).SetString(chainOrderIDText, 10)
	if !ok {
		return Order{}, fmt.Errorf("invalid stored chain_order_id: %s", chainOrderIDText)
	}
	order.ChainOrderID = parsedID
	order.DeliveryBody = deliveryBody

	return order, nil
}

func (s *Store) updateOrder(ctx context.Context, chainOrderID *big.Int, setClause string, args ...any) error {
	values := append(args, time.Now().UTC().Format(time.RFC3339), chainOrderID.String())
	result, err := s.db.ExecContext(
		ctx,
		fmt.Sprintf(`UPDATE orders SET %s, updated_at = ? WHERE chain_order_id = ?`, setClause),
		values...,
	)
	if err != nil {
		return fmt.Errorf("update order: %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("update order rows affected: %w", err)
	}
	if affected == 0 {
		return fmt.Errorf("order not found: %s", chainOrderID.String())
	}

	return nil
}

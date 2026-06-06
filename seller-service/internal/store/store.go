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

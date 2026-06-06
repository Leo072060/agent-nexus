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

// ErrDisputeNotFound is returned by GetDispute when no row matches the order id.
var ErrDisputeNotFound = errors.New("dispute not found")

const disputeColumns = `id, chain_order_id, buyer_address, seller_address, validator_address, request_hash, request_body, delivery_hash, delivery_body, dispute_hash, dispute_body, resolution_hash, resolution_body, release_to_seller, resolve_tx_hash, status, created_at, updated_at`

type rowScanner interface {
	Scan(dest ...any) error
}

type Dispute struct {
	ID               int64
	ChainOrderID     *big.Int
	BuyerAddress     string
	SellerAddress    string
	ValidatorAddress string
	RequestHash      string
	RequestBody      []byte
	DeliveryHash     string
	DeliveryBody     []byte
	DisputeHash      string
	DisputeBody      []byte
	ResolutionHash   string
	ResolutionBody   []byte
	ReleaseToSeller  bool
	ResolveTxHash    string
	Status           string
	CreatedAt        string
	UpdatedAt        string
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
CREATE TABLE IF NOT EXISTS disputes (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	chain_order_id TEXT NOT NULL UNIQUE,
	buyer_address TEXT NOT NULL,
	seller_address TEXT NOT NULL,
	validator_address TEXT NOT NULL,
	request_hash TEXT NOT NULL,
	request_body BLOB NOT NULL,
	delivery_hash TEXT NOT NULL,
	delivery_body BLOB,
	dispute_hash TEXT NOT NULL,
	dispute_body BLOB NOT NULL,
	resolution_hash TEXT NOT NULL DEFAULT '',
	resolution_body BLOB,
	release_to_seller INTEGER NOT NULL DEFAULT 0,
	resolve_tx_hash TEXT NOT NULL DEFAULT '',
	status TEXT NOT NULL,
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL
);
`)
	if err != nil {
		return fmt.Errorf("migrate database: %w", err)
	}

	for _, stmt := range []string{
		`ALTER TABLE disputes ADD COLUMN resolution_hash TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE disputes ADD COLUMN resolution_body BLOB`,
		`ALTER TABLE disputes ADD COLUMN release_to_seller INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE disputes ADD COLUMN resolve_tx_hash TEXT NOT NULL DEFAULT ''`,
	} {
		if _, err := s.db.ExecContext(ctx, stmt); err != nil && !strings.Contains(err.Error(), "duplicate column name") {
			return fmt.Errorf("migrate database: %w", err)
		}
	}

	return nil
}

func (s *Store) UpsertDispute(ctx context.Context, dispute Dispute) (Dispute, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	chainOrderIDText := dispute.ChainOrderID.String()

	_, err := s.db.ExecContext(
		ctx,
		`INSERT INTO disputes (
	chain_order_id,
	buyer_address,
	seller_address,
	validator_address,
	request_hash,
	request_body,
	delivery_hash,
	delivery_body,
	dispute_hash,
	dispute_body,
	status,
	created_at,
	updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(chain_order_id) DO UPDATE SET
	buyer_address = excluded.buyer_address,
	seller_address = excluded.seller_address,
	validator_address = excluded.validator_address,
	request_hash = excluded.request_hash,
	request_body = excluded.request_body,
	delivery_hash = excluded.delivery_hash,
	delivery_body = excluded.delivery_body,
	dispute_hash = excluded.dispute_hash,
	dispute_body = excluded.dispute_body,
	status = excluded.status,
	updated_at = excluded.updated_at`,
		chainOrderIDText,
		dispute.BuyerAddress,
		dispute.SellerAddress,
		dispute.ValidatorAddress,
		dispute.RequestHash,
		dispute.RequestBody,
		dispute.DeliveryHash,
		dispute.DeliveryBody,
		dispute.DisputeHash,
		dispute.DisputeBody,
		dispute.Status,
		now,
		now,
	)
	if err != nil {
		return Dispute{}, fmt.Errorf("upsert dispute: %w", err)
	}

	return s.GetDispute(ctx, dispute.ChainOrderID)
}

func (s *Store) MarkResolved(
	ctx context.Context,
	chainOrderID *big.Int,
	resolutionHash string,
	resolutionBody []byte,
	releaseToSeller bool,
	txHash string,
	status string,
) error {
	now := time.Now().UTC().Format(time.RFC3339)
	releaseToSellerValue := 0
	if releaseToSeller {
		releaseToSellerValue = 1
	}

	result, err := s.db.ExecContext(
		ctx,
		`UPDATE disputes SET resolution_hash = ?, resolution_body = ?, release_to_seller = ?, resolve_tx_hash = ?, status = ?, updated_at = ? WHERE chain_order_id = ?`,
		resolutionHash,
		resolutionBody,
		releaseToSellerValue,
		txHash,
		status,
		now,
		chainOrderID.String(),
	)
	if err != nil {
		return fmt.Errorf("mark resolved: %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("mark resolved rows affected: %w", err)
	}
	if affected == 0 {
		return fmt.Errorf("dispute not found: %s", chainOrderID.String())
	}

	return nil
}

func (s *Store) GetDispute(ctx context.Context, chainOrderID *big.Int) (Dispute, error) {
	row := s.db.QueryRowContext(
		ctx,
		`SELECT `+disputeColumns+` FROM disputes WHERE chain_order_id = ?`,
		chainOrderID.String(),
	)

	dispute, err := scanDispute(row)
	if errors.Is(err, sql.ErrNoRows) {
		return Dispute{}, fmt.Errorf("%w: %s", ErrDisputeNotFound, chainOrderID.String())
	}
	if err != nil {
		return Dispute{}, fmt.Errorf("get dispute: %w", err)
	}

	return dispute, nil
}

// ListDisputes returns every stored dispute ordered by numeric order id ascending.
func (s *Store) ListDisputes(ctx context.Context) ([]Dispute, error) {
	rows, err := s.db.QueryContext(
		ctx,
		`SELECT `+disputeColumns+` FROM disputes ORDER BY CAST(chain_order_id AS INTEGER) ASC`,
	)
	if err != nil {
		return nil, fmt.Errorf("list disputes: %w", err)
	}
	defer rows.Close()

	disputes := make([]Dispute, 0)
	for rows.Next() {
		dispute, err := scanDispute(rows)
		if err != nil {
			return nil, fmt.Errorf("list disputes: %w", err)
		}
		disputes = append(disputes, dispute)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list disputes: %w", err)
	}

	return disputes, nil
}

func scanDispute(scanner rowScanner) (Dispute, error) {
	var (
		dispute          Dispute
		chainOrderIDText string
		releaseToSeller  int
	)

	if err := scanner.Scan(
		&dispute.ID,
		&chainOrderIDText,
		&dispute.BuyerAddress,
		&dispute.SellerAddress,
		&dispute.ValidatorAddress,
		&dispute.RequestHash,
		&dispute.RequestBody,
		&dispute.DeliveryHash,
		&dispute.DeliveryBody,
		&dispute.DisputeHash,
		&dispute.DisputeBody,
		&dispute.ResolutionHash,
		&dispute.ResolutionBody,
		&releaseToSeller,
		&dispute.ResolveTxHash,
		&dispute.Status,
		&dispute.CreatedAt,
		&dispute.UpdatedAt,
	); err != nil {
		return Dispute{}, err
	}

	parsedID, ok := new(big.Int).SetString(chainOrderIDText, 10)
	if !ok {
		return Dispute{}, fmt.Errorf("invalid stored chain_order_id: %s", chainOrderIDText)
	}
	dispute.ChainOrderID = parsedID
	dispute.ReleaseToSeller = releaseToSeller == 1

	return dispute, nil
}

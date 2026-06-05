package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
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

type CreateRequestInput struct {
	RequestHash string
	Content     string
	ContentPath string
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
`)
	if err != nil {
		return fmt.Errorf("migrate database: %w", err)
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

package mysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/devpedrois/snip/internal/domain"
)

// URLRepository defines the contract for URL persistence.
type URLRepository interface {
	Create(ctx context.Context, u *domain.URL) error
	FindByHash(ctx context.Context, hash string) (*domain.URL, error)
	FindByID(ctx context.Context, id uint64) (*domain.URL, error)
	UpdateHash(ctx context.Context, id uint64, hash string) error
	UpdateLastAccessed(ctx context.Context, id uint64) error
}

// MySQLURLRepository implements URLRepository using *sql.DB.
type MySQLURLRepository struct {
	db *sql.DB
}

// NewURLRepository returns a new MySQLURLRepository.
func NewURLRepository(db *sql.DB) URLRepository {
	return &MySQLURLRepository{db: db}
}

func (r *MySQLURLRepository) Create(ctx context.Context, u *domain.URL) error {
	const q = `INSERT INTO urls (hash, original_url, expires_at) VALUES (?, ?, ?)`

	res, err := r.db.ExecContext(ctx, q, u.Hash, u.OriginalURL, u.ExpiresAt)
	if err != nil {
		return fmt.Errorf("url_repository: create: %w", err)
	}

	id, err := res.LastInsertId()
	if err != nil {
		return fmt.Errorf("url_repository: last insert id: %w", err)
	}

	u.ID = uint64(id)
	return nil
}

func (r *MySQLURLRepository) FindByHash(ctx context.Context, hash string) (*domain.URL, error) {
	const q = `SELECT id, hash, original_url, created_at, last_accessed_at, expires_at FROM urls WHERE hash = ?`

	u := &domain.URL{}
	err := r.db.QueryRowContext(ctx, q, hash).Scan(
		&u.ID, &u.Hash, &u.OriginalURL, &u.CreatedAt, &u.LastAccessedAt, &u.ExpiresAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrURLNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("url_repository: find by hash: %w", err)
	}

	return u, nil
}

func (r *MySQLURLRepository) FindByID(ctx context.Context, id uint64) (*domain.URL, error) {
	const q = `SELECT id, hash, original_url, created_at, last_accessed_at, expires_at FROM urls WHERE id = ?`

	u := &domain.URL{}
	err := r.db.QueryRowContext(ctx, q, id).Scan(
		&u.ID, &u.Hash, &u.OriginalURL, &u.CreatedAt, &u.LastAccessedAt, &u.ExpiresAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrURLNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("url_repository: find by id: %w", err)
	}

	return u, nil
}

func (r *MySQLURLRepository) UpdateHash(ctx context.Context, id uint64, hash string) error {
	const q = `UPDATE urls SET hash = ? WHERE id = ?`

	_, err := r.db.ExecContext(ctx, q, hash, id)
	if err != nil {
		return fmt.Errorf("url_repository: update hash: %w", err)
	}

	return nil
}

func (r *MySQLURLRepository) UpdateLastAccessed(ctx context.Context, id uint64) error {
	const q = `UPDATE urls SET last_accessed_at = NOW() WHERE id = ?`

	_, err := r.db.ExecContext(ctx, q, id)
	if err != nil {
		return fmt.Errorf("url_repository: update last accessed: %w", err)
	}

	return nil
}

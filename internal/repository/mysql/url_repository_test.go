package mysql_test

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/devpedrois/snip/internal/domain"
	"github.com/devpedrois/snip/internal/repository/mysql"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newURLRepo(t *testing.T) (mysql.URLRepository, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })
	return mysql.NewURLRepository(db), mock
}

func TestURLRepository_Create(t *testing.T) {
	now := time.Now()
	expiresAt := now.Add(30 * 24 * time.Hour)

	tests := []struct {
		name    string
		url     *domain.URL
		setup   func(mock sqlmock.Sqlmock)
		wantID  uint64
		wantErr bool
	}{
		{
			name: "success",
			url:  &domain.URL{Hash: "abc1234", OriginalURL: "https://example.com", ExpiresAt: &expiresAt},
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec("INSERT INTO urls (hash, original_url, expires_at) VALUES (?, ?, ?)").
					WithArgs("abc1234", "https://example.com", &expiresAt).
					WillReturnResult(sqlmock.NewResult(42, 1))
			},
			wantID: 42,
		},
		{
			name: "sql error",
			url:  &domain.URL{Hash: "abc1234", OriginalURL: "https://example.com", ExpiresAt: &expiresAt},
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec("INSERT INTO urls (hash, original_url, expires_at) VALUES (?, ?, ?)").
					WithArgs("abc1234", "https://example.com", &expiresAt).
					WillReturnError(errors.New("db error"))
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo, mock := newURLRepo(t)
			tt.setup(mock)

			err := repo.Create(context.Background(), tt.url)

			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantID, tt.url.ID)
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestURLRepository_FindByHash(t *testing.T) {
	now := time.Now().Truncate(time.Second)
	cols := []string{"id", "hash", "original_url", "created_at", "last_accessed_at", "expires_at"}

	tests := []struct {
		name    string
		hash    string
		setup   func(mock sqlmock.Sqlmock)
		wantURL *domain.URL
		wantErr error
	}{
		{
			name: "success",
			hash: "abc1234",
			setup: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows(cols).
					AddRow(uint64(1), "abc1234", "https://example.com", now, nil, nil)
				mock.ExpectQuery("SELECT id, hash, original_url, created_at, last_accessed_at, expires_at FROM urls WHERE hash = ?").
					WithArgs("abc1234").
					WillReturnRows(rows)
			},
			wantURL: &domain.URL{ID: 1, Hash: "abc1234", OriginalURL: "https://example.com", CreatedAt: now},
		},
		{
			name: "not found",
			hash: "missing",
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery("SELECT id, hash, original_url, created_at, last_accessed_at, expires_at FROM urls WHERE hash = ?").
					WithArgs("missing").
					WillReturnError(sql.ErrNoRows)
			},
			wantErr: domain.ErrURLNotFound,
		},
		{
			name: "sql error",
			hash: "abc1234",
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery("SELECT id, hash, original_url, created_at, last_accessed_at, expires_at FROM urls WHERE hash = ?").
					WithArgs("abc1234").
					WillReturnError(errors.New("connection refused"))
			},
			wantErr: errors.New("url_repository: find by hash"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo, mock := newURLRepo(t)
			tt.setup(mock)

			got, err := repo.FindByHash(context.Background(), tt.hash)

			if tt.wantErr != nil {
				require.Error(t, err)
				if errors.Is(tt.wantErr, domain.ErrURLNotFound) {
					assert.ErrorIs(t, err, domain.ErrURLNotFound)
				}
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantURL.ID, got.ID)
			assert.Equal(t, tt.wantURL.Hash, got.Hash)
			assert.Equal(t, tt.wantURL.OriginalURL, got.OriginalURL)
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestURLRepository_FindByID(t *testing.T) {
	now := time.Now().Truncate(time.Second)
	cols := []string{"id", "hash", "original_url", "created_at", "last_accessed_at", "expires_at"}

	tests := []struct {
		name    string
		id      uint64
		setup   func(mock sqlmock.Sqlmock)
		wantURL *domain.URL
		wantErr error
	}{
		{
			name: "success",
			id:   1,
			setup: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows(cols).
					AddRow(uint64(1), "abc1234", "https://example.com", now, nil, nil)
				mock.ExpectQuery("SELECT id, hash, original_url, created_at, last_accessed_at, expires_at FROM urls WHERE id = ?").
					WithArgs(uint64(1)).
					WillReturnRows(rows)
			},
			wantURL: &domain.URL{ID: 1, Hash: "abc1234", OriginalURL: "https://example.com"},
		},
		{
			name: "not found",
			id:   999,
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery("SELECT id, hash, original_url, created_at, last_accessed_at, expires_at FROM urls WHERE id = ?").
					WithArgs(uint64(999)).
					WillReturnError(sql.ErrNoRows)
			},
			wantErr: domain.ErrURLNotFound,
		},
		{
			name: "sql error",
			id:   1,
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery("SELECT id, hash, original_url, created_at, last_accessed_at, expires_at FROM urls WHERE id = ?").
					WithArgs(uint64(1)).
					WillReturnError(errors.New("connection lost"))
			},
			wantErr: errors.New("url_repository: find by id"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo, mock := newURLRepo(t)
			tt.setup(mock)

			got, err := repo.FindByID(context.Background(), tt.id)

			if tt.wantErr != nil {
				require.Error(t, err)
				if errors.Is(tt.wantErr, domain.ErrURLNotFound) {
					assert.ErrorIs(t, err, domain.ErrURLNotFound)
				}
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantURL.ID, got.ID)
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestURLRepository_UpdateHash(t *testing.T) {
	tests := []struct {
		name    string
		id      uint64
		hash    string
		setup   func(mock sqlmock.Sqlmock)
		wantErr bool
	}{
		{
			name: "success",
			id:   1,
			hash: "xyz9999",
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec("UPDATE urls SET hash = ? WHERE id = ?").
					WithArgs("xyz9999", uint64(1)).
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
		},
		{
			name: "sql error",
			id:   1,
			hash: "xyz9999",
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec("UPDATE urls SET hash = ? WHERE id = ?").
					WithArgs("xyz9999", uint64(1)).
					WillReturnError(errors.New("db error"))
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo, mock := newURLRepo(t)
			tt.setup(mock)

			err := repo.UpdateHash(context.Background(), tt.id, tt.hash)

			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestURLRepository_UpdateLastAccessed(t *testing.T) {
	tests := []struct {
		name    string
		id      uint64
		setup   func(mock sqlmock.Sqlmock)
		wantErr bool
	}{
		{
			name: "success",
			id:   1,
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec("UPDATE urls SET last_accessed_at = NOW() WHERE id = ?").
					WithArgs(uint64(1)).
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
		},
		{
			name: "sql error",
			id:   1,
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec("UPDATE urls SET last_accessed_at = NOW() WHERE id = ?").
					WithArgs(uint64(1)).
					WillReturnError(errors.New("db error"))
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo, mock := newURLRepo(t)
			tt.setup(mock)

			err := repo.UpdateLastAccessed(context.Background(), tt.id)

			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

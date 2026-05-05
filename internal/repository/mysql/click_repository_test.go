package mysql_test

import (
	"context"
	"errors"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/devpedrois/snip/internal/domain"
	"github.com/devpedrois/snip/internal/repository/mysql"
)

func newClickRepo(t *testing.T) (mysql.ClickRepository, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() }) //nolint:errcheck,gosec
	return mysql.NewClickRepository(db), mock
}

func TestClickRepository_Insert(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name    string
		click   *domain.Click
		setup   func(mock sqlmock.Sqlmock)
		wantID  uint64
		wantErr bool
	}{
		{
			name:  "success",
			click: &domain.Click{URLID: 1, AccessedAt: now, UserAgent: "Mozilla/5.0", IP: "127.0.0.1"},
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec("INSERT INTO clicks (url_id, accessed_at, user_agent, ip) VALUES (?, ?, ?, ?)").
					WithArgs(uint64(1), now, "Mozilla/5.0", "127.0.0.1").
					WillReturnResult(sqlmock.NewResult(10, 1))
			},
			wantID: 10,
		},
		{
			name:  "sql error",
			click: &domain.Click{URLID: 1, AccessedAt: now, UserAgent: "Mozilla/5.0", IP: "127.0.0.1"},
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec("INSERT INTO clicks (url_id, accessed_at, user_agent, ip) VALUES (?, ?, ?, ?)").
					WithArgs(uint64(1), now, "Mozilla/5.0", "127.0.0.1").
					WillReturnError(errors.New("db error"))
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo, mock := newClickRepo(t)
			tt.setup(mock)

			err := repo.Insert(context.Background(), tt.click)

			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantID, tt.click.ID)
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestClickRepository_CountByURLID(t *testing.T) {
	tests := []struct {
		name      string
		urlID     uint64
		setup     func(mock sqlmock.Sqlmock)
		wantCount int64
		wantErr   bool
	}{
		{
			name:  "success",
			urlID: 1,
			setup: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{"count"}).AddRow(int64(42))
				mock.ExpectQuery("SELECT COUNT(*) FROM clicks WHERE url_id = ?").
					WithArgs(uint64(1)).
					WillReturnRows(rows)
			},
			wantCount: 42,
		},
		{
			name:  "zero count",
			urlID: 99,
			setup: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{"count"}).AddRow(int64(0))
				mock.ExpectQuery("SELECT COUNT(*) FROM clicks WHERE url_id = ?").
					WithArgs(uint64(99)).
					WillReturnRows(rows)
			},
			wantCount: 0,
		},
		{
			name:  "sql error",
			urlID: 1,
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery("SELECT COUNT(*) FROM clicks WHERE url_id = ?").
					WithArgs(uint64(1)).
					WillReturnError(errors.New("db error"))
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo, mock := newClickRepo(t)
			tt.setup(mock)

			count, err := repo.CountByURLID(context.Background(), tt.urlID)

			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantCount, count)
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestClickRepository_GroupByDay(t *testing.T) {
	const groupByDayQuery = `
		SELECT DATE(accessed_at) AS date, COUNT(*) AS count
		FROM clicks
		WHERE url_id = ? AND accessed_at >= DATE_SUB(NOW(), INTERVAL ? DAY)
		GROUP BY DATE(accessed_at)
		ORDER BY date DESC`

	tests := []struct {
		name    string
		urlID   uint64
		days    int
		setup   func(mock sqlmock.Sqlmock)
		want    []domain.DailyCount
		wantErr bool
	}{
		{
			name:  "success with rows",
			urlID: 1,
			days:  30,
			setup: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{"date", "count"}).
					AddRow("2026-05-01", int64(5)).
					AddRow("2026-04-30", int64(3))
				mock.ExpectQuery(groupByDayQuery).
					WithArgs(uint64(1), 30).
					WillReturnRows(rows)
			},
			want: []domain.DailyCount{
				{Date: "2026-05-01", Count: 5},
				{Date: "2026-04-30", Count: 3},
			},
		},
		{
			name:  "empty result",
			urlID: 1,
			days:  30,
			setup: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{"date", "count"})
				mock.ExpectQuery(groupByDayQuery).
					WithArgs(uint64(1), 30).
					WillReturnRows(rows)
			},
			want: nil,
		},
		{
			name:  "sql error",
			urlID: 1,
			days:  30,
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(groupByDayQuery).
					WithArgs(uint64(1), 30).
					WillReturnError(errors.New("db error"))
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo, mock := newClickRepo(t)
			tt.setup(mock)

			got, err := repo.GroupByDay(context.Background(), tt.urlID, tt.days)

			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestClickRepository_TopUserAgents(t *testing.T) {
	const topUAQuery = `
		SELECT COALESCE(user_agent, '') AS user_agent, COUNT(*) AS count
		FROM clicks
		WHERE url_id = ?
		GROUP BY user_agent
		ORDER BY count DESC
		LIMIT ?`

	tests := []struct {
		name    string
		urlID   uint64
		limit   int
		setup   func(mock sqlmock.Sqlmock)
		want    []domain.UserAgentCount
		wantErr bool
	}{
		{
			name:  "success with rows",
			urlID: 1,
			limit: 3,
			setup: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{"user_agent", "count"}).
					AddRow("Mozilla/5.0", int64(10)).
					AddRow("curl/7.68", int64(4))
				mock.ExpectQuery(topUAQuery).
					WithArgs(uint64(1), 3).
					WillReturnRows(rows)
			},
			want: []domain.UserAgentCount{
				{UserAgent: "Mozilla/5.0", Count: 10},
				{UserAgent: "curl/7.68", Count: 4},
			},
		},
		{
			name:  "empty result",
			urlID: 1,
			limit: 5,
			setup: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{"user_agent", "count"})
				mock.ExpectQuery(topUAQuery).
					WithArgs(uint64(1), 5).
					WillReturnRows(rows)
			},
			want: nil,
		},
		{
			name:  "sql error",
			urlID: 1,
			limit: 5,
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(topUAQuery).
					WithArgs(uint64(1), 5).
					WillReturnError(errors.New("db error"))
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo, mock := newClickRepo(t)
			tt.setup(mock)

			got, err := repo.TopUserAgents(context.Background(), tt.urlID, tt.limit)

			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestClickRepository_DeleteOlderThan(t *testing.T) {
	const deleteQuery = `DELETE FROM clicks WHERE accessed_at < NOW() - INTERVAL ? DAY LIMIT 10000`

	tests := []struct {
		name        string
		days        int
		setup       func(mock sqlmock.Sqlmock)
		wantDeleted int64
		wantErr     bool
	}{
		{
			name: "deletes old clicks",
			days: 90,
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(deleteQuery).
					WithArgs(90).
					WillReturnResult(sqlmock.NewResult(0, 42))
			},
			wantDeleted: 42,
		},
		{
			name: "no rows to delete",
			days: 90,
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(deleteQuery).
					WithArgs(90).
					WillReturnResult(sqlmock.NewResult(0, 0))
			},
			wantDeleted: 0,
		},
		{
			name: "sql error",
			days: 90,
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(deleteQuery).
					WithArgs(90).
					WillReturnError(errors.New("db error"))
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo, mock := newClickRepo(t)
			tt.setup(mock)

			deleted, err := repo.DeleteOlderThan(context.Background(), tt.days)

			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantDeleted, deleted)
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

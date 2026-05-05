package mysql_test

import (
	"context"
	"errors"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/devpedrois/snip/internal/domain"
	"github.com/devpedrois/snip/internal/repository/mysql"
)

func newReportRepo(t *testing.T) (*mysql.MySQLReportRepository, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })
	return mysql.NewReportRepository(db), mock
}

func TestReportRepository_Insert(t *testing.T) {
	const q = `INSERT INTO url_reports (url_id, reporter_ip, reason) VALUES (?, ?, ?)`

	tests := []struct {
		name    string
		report  *domain.Report
		setup   func(mock sqlmock.Sqlmock)
		wantID  uint64
		wantErr error
	}{
		{
			name:   "success",
			report: &domain.Report{URLID: 1, ReporterIP: "192.168.1.0", Reason: "phishing"},
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(q).
					WithArgs(uint64(1), "192.168.1.0", "phishing").
					WillReturnResult(sqlmock.NewResult(10, 1))
			},
			wantID: 10,
		},
		{
			name:   "duplicate entry returns ErrDuplicateReport",
			report: &domain.Report{URLID: 1, ReporterIP: "192.168.1.0", Reason: "spam"},
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(q).
					WithArgs(uint64(1), "192.168.1.0", "spam").
					WillReturnError(errors.New("Duplicate entry '1-192.168.1.0' for key 'uk_url_reporter'"))
			},
			wantErr: domain.ErrDuplicateReport,
		},
		{
			name:   "generic db error",
			report: &domain.Report{URLID: 2, ReporterIP: "10.0.0.0", Reason: "malware"},
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(q).
					WithArgs(uint64(2), "10.0.0.0", "malware").
					WillReturnError(errors.New("connection refused"))
			},
			wantErr: errors.New("connection refused"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo, mock := newReportRepo(t)
			tt.setup(mock)

			err := repo.Insert(context.Background(), tt.report)

			if tt.wantErr != nil {
				require.Error(t, err)
				if errors.Is(tt.wantErr, domain.ErrDuplicateReport) {
					assert.ErrorIs(t, err, domain.ErrDuplicateReport)
				}
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantID, tt.report.ID)
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestReportRepository_CountDistinctIPsByURLID(t *testing.T) {
	const q = `SELECT COUNT(DISTINCT reporter_ip) FROM url_reports WHERE url_id = ?`

	tests := []struct {
		name      string
		urlID     uint64
		setup     func(mock sqlmock.Sqlmock)
		wantCount int64
		wantErr   bool
	}{
		{
			name:  "returns correct count",
			urlID: 5,
			setup: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{"count"}).AddRow(int64(3))
				mock.ExpectQuery(q).WithArgs(uint64(5)).WillReturnRows(rows)
			},
			wantCount: 3,
		},
		{
			name:  "zero reports",
			urlID: 9,
			setup: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{"count"}).AddRow(int64(0))
				mock.ExpectQuery(q).WithArgs(uint64(9)).WillReturnRows(rows)
			},
			wantCount: 0,
		},
		{
			name:  "db error",
			urlID: 1,
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(q).WithArgs(uint64(1)).WillReturnError(errors.New("db error"))
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo, mock := newReportRepo(t)
			tt.setup(mock)

			count, err := repo.CountDistinctIPsByURLID(context.Background(), tt.urlID)

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

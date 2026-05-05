package mysql

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/devpedrois/snip/internal/domain"
)

// ReportRepository defines the contract for abuse report persistence.
type ReportRepository interface {
	Insert(ctx context.Context, r *domain.Report) error
	CountDistinctIPsByURLID(ctx context.Context, urlID uint64) (int64, error)
}

// MySQLReportRepository implements ReportRepository using *sql.DB.
type MySQLReportRepository struct {
	db *sql.DB
}

// NewReportRepository returns a new MySQLReportRepository.
func NewReportRepository(db *sql.DB) *MySQLReportRepository {
	return &MySQLReportRepository{db: db}
}

func (r *MySQLReportRepository) Insert(ctx context.Context, rep *domain.Report) error {
	const q = `INSERT INTO url_reports (url_id, reporter_ip, reason) VALUES (?, ?, ?)`

	res, err := r.db.ExecContext(ctx, q, rep.URLID, rep.ReporterIP, rep.Reason)
	if err != nil {
		if isDuplicateEntry(err) {
			return domain.ErrDuplicateReport
		}
		return fmt.Errorf("report_repository: insert: %w", err)
	}

	id, err := res.LastInsertId()
	if err != nil {
		return fmt.Errorf("report_repository: last insert id: %w", err)
	}

	rep.ID = uint64(id)
	return nil
}

func (r *MySQLReportRepository) CountDistinctIPsByURLID(ctx context.Context, urlID uint64) (int64, error) {
	const q = `SELECT COUNT(DISTINCT reporter_ip) FROM url_reports WHERE url_id = ?`

	var count int64
	if err := r.db.QueryRowContext(ctx, q, urlID).Scan(&count); err != nil {
		return 0, fmt.Errorf("report_repository: count distinct ips: %w", err)
	}

	return count, nil
}

func isDuplicateEntry(err error) bool {
	return strings.Contains(err.Error(), "Duplicate entry") || strings.Contains(err.Error(), "duplicate entry")
}

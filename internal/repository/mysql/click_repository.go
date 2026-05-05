package mysql

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/devpedrois/snip/internal/domain"
)

// ClickRepository defines the contract for click persistence.
type ClickRepository interface {
	Insert(ctx context.Context, c *domain.Click) error
	CountByURLID(ctx context.Context, urlID uint64) (int64, error)
	GroupByDay(ctx context.Context, urlID uint64, days int) ([]domain.DailyCount, error)
	TopUserAgents(ctx context.Context, urlID uint64, limit int) ([]domain.UserAgentCount, error)
	DeleteOlderThan(ctx context.Context, days int) (int64, error)
}

// MySQLClickRepository implements ClickRepository using *sql.DB.
type MySQLClickRepository struct {
	db *sql.DB
}

// NewClickRepository returns a new MySQLClickRepository.
func NewClickRepository(db *sql.DB) *MySQLClickRepository {
	return &MySQLClickRepository{db: db}
}

func (r *MySQLClickRepository) Insert(ctx context.Context, c *domain.Click) error {
	const q = `INSERT INTO clicks (url_id, accessed_at, user_agent, ip) VALUES (?, ?, ?, ?)`

	res, err := r.db.ExecContext(ctx, q, c.URLID, c.AccessedAt, c.UserAgent, c.IP)
	if err != nil {
		return fmt.Errorf("click_repository: insert: %w", err)
	}

	id, err := res.LastInsertId()
	if err != nil {
		return fmt.Errorf("click_repository: last insert id: %w", err)
	}

	c.ID = uint64(id)
	return nil
}

func (r *MySQLClickRepository) CountByURLID(ctx context.Context, urlID uint64) (int64, error) {
	const q = `SELECT COUNT(*) FROM clicks WHERE url_id = ?`

	var count int64
	err := r.db.QueryRowContext(ctx, q, urlID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("click_repository: count by url id: %w", err)
	}

	return count, nil
}

func (r *MySQLClickRepository) GroupByDay(ctx context.Context, urlID uint64, days int) ([]domain.DailyCount, error) {
	const q = `
		SELECT DATE(accessed_at) AS date, COUNT(*) AS count
		FROM clicks
		WHERE url_id = ? AND accessed_at >= DATE_SUB(NOW(), INTERVAL ? DAY)
		GROUP BY DATE(accessed_at)
		ORDER BY date DESC`

	rows, err := r.db.QueryContext(ctx, q, urlID, days)
	if err != nil {
		return nil, fmt.Errorf("click_repository: group by day: %w", err)
	}
	defer rows.Close()

	var results []domain.DailyCount
	for rows.Next() {
		var dc domain.DailyCount
		if err := rows.Scan(&dc.Date, &dc.Count); err != nil {
			return nil, fmt.Errorf("click_repository: group by day scan: %w", err)
		}
		results = append(results, dc)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("click_repository: group by day rows: %w", err)
	}

	return results, nil
}

func (r *MySQLClickRepository) TopUserAgents(ctx context.Context, urlID uint64, limit int) ([]domain.UserAgentCount, error) {
	const q = `
		SELECT COALESCE(user_agent, '') AS user_agent, COUNT(*) AS count
		FROM clicks
		WHERE url_id = ?
		GROUP BY user_agent
		ORDER BY count DESC
		LIMIT ?`

	rows, err := r.db.QueryContext(ctx, q, urlID, limit)
	if err != nil {
		return nil, fmt.Errorf("click_repository: top user agents: %w", err)
	}
	defer rows.Close()

	var results []domain.UserAgentCount
	for rows.Next() {
		var ua domain.UserAgentCount
		if err := rows.Scan(&ua.UserAgent, &ua.Count); err != nil {
			return nil, fmt.Errorf("click_repository: top user agents scan: %w", err)
		}
		results = append(results, ua)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("click_repository: top user agents rows: %w", err)
	}

	return results, nil
}

func (r *MySQLClickRepository) DeleteOlderThan(ctx context.Context, days int) (int64, error) {
	const q = `DELETE FROM clicks WHERE accessed_at < NOW() - INTERVAL ? DAY LIMIT 10000`

	res, err := r.db.ExecContext(ctx, q, days)
	if err != nil {
		return 0, fmt.Errorf("click_repository: delete older than: %w", err)
	}

	n, err := res.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("click_repository: delete older than rows affected: %w", err)
	}

	return n, nil
}

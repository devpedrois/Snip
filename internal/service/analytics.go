package service

import (
	"context"
	"fmt"

	"github.com/devpedrois/snip/internal/domain"
)

type URLFinder interface {
	FindByHash(ctx context.Context, hash string) (*domain.URL, error)
}

type ClickReader interface {
	CountByURLID(ctx context.Context, urlID uint64) (int64, error)
	GroupByDay(ctx context.Context, urlID uint64, days int) ([]domain.DailyCount, error)
	TopUserAgents(ctx context.Context, urlID uint64, limit int) ([]domain.UserAgentCount, error)
}

type Stats struct {
	TotalClicks   int64                   `json:"total_clicks"`
	ClicksByDay   []domain.DailyCount     `json:"clicks_by_day"`
	TopUserAgents []domain.UserAgentCount `json:"top_user_agents"`
}

type AnalyticsService interface {
	GetStats(ctx context.Context, hash string) (*Stats, error)
}

type analyticsService struct {
	urlRepo   URLFinder
	clickRepo ClickReader
}

func NewAnalyticsService(urlRepo URLFinder, clickRepo ClickReader) AnalyticsService {
	return &analyticsService{urlRepo: urlRepo, clickRepo: clickRepo}
}

func (s *analyticsService) GetStats(ctx context.Context, hash string) (*Stats, error) {
	u, err := s.urlRepo.FindByHash(ctx, hash)
	if err != nil {
		return nil, fmt.Errorf("analytics: find url: %w", err)
	}

	total, err := s.clickRepo.CountByURLID(ctx, u.ID)
	if err != nil {
		return nil, fmt.Errorf("analytics: count clicks: %w", err)
	}

	byDay, err := s.clickRepo.GroupByDay(ctx, u.ID, 30)
	if err != nil {
		return nil, fmt.Errorf("analytics: group by day: %w", err)
	}
	if byDay == nil {
		byDay = []domain.DailyCount{}
	}

	topAgents, err := s.clickRepo.TopUserAgents(ctx, u.ID, 5)
	if err != nil {
		return nil, fmt.Errorf("analytics: top user agents: %w", err)
	}
	if topAgents == nil {
		topAgents = []domain.UserAgentCount{}
	}

	return &Stats{
		TotalClicks:   total,
		ClicksByDay:   byDay,
		TopUserAgents: topAgents,
	}, nil
}

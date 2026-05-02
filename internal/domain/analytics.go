package domain

// DailyCount holds the click count for a specific day.
type DailyCount struct {
	Date  string
	Count int64
}

// UserAgentCount holds the click count for a specific user-agent.
type UserAgentCount struct {
	UserAgent string
	Count     int64
}

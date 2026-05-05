package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	AppPort string
	AppEnv  string
	BaseURL string

	MySQLHost         string
	MySQLPort         string
	MySQLUser         string
	MySQLPassword     string
	MySQLDatabase     string
	MySQLMaxOpenConns int
	MySQLMaxIdleConns int

	RedisHost     string
	RedisPort     string
	RedisPassword string
	RedisDB       int
	RedisTTLDays  int

	AnalyticsWorkers int
	AnalyticsBuffer  int

	URLExpirationDays           int
	BackgroundUpdateTimeoutSecs int

	AllowedOrigins     string
	TrustedProxies     string
	RateLimitShorten   int
	RateLimitRedirect  int
	RateLimitAnalytics int
	RateLimitReport    int

	AnalyticsRetentionDays int
	HealthSecretKey        string

	ReportAutoBlockThreshold int

	VTEnabled             bool
	VTAPIKey              string
	VTTimeoutSeconds      time.Duration
	VTMinPositives        int
	VTCacheTTLHours       int
	VTRescanIntervalHours int
}

// LoadConfig loads configuration from environment variables, optionally from a .env file.
func LoadConfig() (*Config, error) {
	_ = godotenv.Load()

	cfg := &Config{}

	cfg.AppPort = envOrDefault("APP_PORT", "8080")
	cfg.AppEnv = envOrDefault("APP_ENV", "development")
	cfg.BaseURL = envOrDefault("BASE_URL", "http://localhost:8080")

	var missing []string
	var parseErrs []string

	cfg.MySQLHost = requireEnv("MYSQL_HOST", &missing)
	cfg.MySQLPort = envOrDefault("MYSQL_PORT", "3306")
	cfg.MySQLUser = requireEnv("MYSQL_USER", &missing)
	cfg.MySQLPassword = requireEnv("MYSQL_PASSWORD", &missing)
	cfg.MySQLDatabase = requireEnv("MYSQL_DATABASE", &missing)

	var err error
	if cfg.MySQLMaxOpenConns, err = parseIntEnv("MYSQL_MAX_OPEN_CONNS", 25); err != nil {
		parseErrs = append(parseErrs, err.Error())
	}
	if cfg.MySQLMaxIdleConns, err = parseIntEnv("MYSQL_MAX_IDLE_CONNS", 10); err != nil {
		parseErrs = append(parseErrs, err.Error())
	}

	cfg.RedisHost = requireEnv("REDIS_HOST", &missing)
	cfg.RedisPort = envOrDefault("REDIS_PORT", "6379")
	cfg.RedisPassword = os.Getenv("REDIS_PASSWORD")
	if cfg.RedisDB, err = parseIntEnv("REDIS_DB", 0); err != nil {
		parseErrs = append(parseErrs, err.Error())
	}
	if cfg.RedisTTLDays, err = parseIntEnv("REDIS_TTL_DAYS", 30); err != nil {
		parseErrs = append(parseErrs, err.Error())
	}
	if cfg.AnalyticsWorkers, err = parseIntEnv("ANALYTICS_WORKERS", 4); err != nil {
		parseErrs = append(parseErrs, err.Error())
	}
	if cfg.AnalyticsBuffer, err = parseIntEnv("ANALYTICS_BUFFER", 1000); err != nil {
		parseErrs = append(parseErrs, err.Error())
	}
	if cfg.URLExpirationDays, err = parseIntEnv("URL_EXPIRATION_DAYS", 30); err != nil {
		parseErrs = append(parseErrs, err.Error())
	}
	if cfg.BackgroundUpdateTimeoutSecs, err = parseIntEnv("BACKGROUND_UPDATE_TIMEOUT_SECS", 2); err != nil {
		parseErrs = append(parseErrs, err.Error())
	}

	cfg.AllowedOrigins = envOrDefault("ALLOWED_ORIGINS", "*")
	cfg.TrustedProxies = os.Getenv("TRUSTED_PROXIES")

	if cfg.RateLimitShorten, err = parseIntEnv("RATE_LIMIT_SHORTEN", 5); err != nil {
		parseErrs = append(parseErrs, err.Error())
	}
	if cfg.RateLimitRedirect, err = parseIntEnv("RATE_LIMIT_REDIRECT", 60); err != nil {
		parseErrs = append(parseErrs, err.Error())
	}
	if cfg.RateLimitAnalytics, err = parseIntEnv("RATE_LIMIT_ANALYTICS", 30); err != nil {
		parseErrs = append(parseErrs, err.Error())
	}
	if cfg.RateLimitReport, err = parseIntEnv("RATE_LIMIT_REPORT", 3); err != nil {
		parseErrs = append(parseErrs, err.Error())
	}
	if cfg.AnalyticsRetentionDays, err = parseIntEnv("ANALYTICS_RETENTION_DAYS", 90); err != nil {
		parseErrs = append(parseErrs, err.Error())
	}
	cfg.HealthSecretKey = os.Getenv("HEALTH_SECRET_KEY")

	if cfg.ReportAutoBlockThreshold, err = parseIntEnv("REPORT_AUTO_BLOCK_THRESHOLD", 5); err != nil {
		parseErrs = append(parseErrs, err.Error())
	}

	cfg.VTEnabled = envOrDefault("VT_ENABLED", "true") != "false"
	cfg.VTAPIKey = os.Getenv("VIRUSTOTAL_API_KEY")

	vtTimeoutSecs, err := parseIntEnv("VT_TIMEOUT_SECONDS", 10)
	if err != nil {
		parseErrs = append(parseErrs, err.Error())
	}
	cfg.VTTimeoutSeconds = time.Duration(vtTimeoutSecs) * time.Second

	if cfg.VTMinPositives, err = parseIntEnv("VT_MIN_POSITIVES", 2); err != nil {
		parseErrs = append(parseErrs, err.Error())
	}
	if cfg.VTCacheTTLHours, err = parseIntEnv("VT_CACHE_TTL_HOURS", 24); err != nil {
		parseErrs = append(parseErrs, err.Error())
	}
	if cfg.VTRescanIntervalHours, err = parseIntEnv("VT_RESCAN_INTERVAL_HOURS", 6); err != nil {
		parseErrs = append(parseErrs, err.Error())
	}

	if len(missing) > 0 {
		return nil, fmt.Errorf("config: missing required env vars: %s", strings.Join(missing, ", "))
	}
	if len(parseErrs) > 0 {
		return nil, fmt.Errorf("config: invalid env vars: %s", strings.Join(parseErrs, "; "))
	}
	if cfg.MySQLMaxIdleConns > cfg.MySQLMaxOpenConns {
		return nil, fmt.Errorf("config: MYSQL_MAX_IDLE_CONNS (%d) must not exceed MYSQL_MAX_OPEN_CONNS (%d)",
			cfg.MySQLMaxIdleConns, cfg.MySQLMaxOpenConns)
	}

	return cfg, nil
}

// DSN returns the MySQL data source name for regular connections.
func (c *Config) DSN() string {
	return fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?parseTime=true",
		c.MySQLUser, c.MySQLPassword, c.MySQLHost, c.MySQLPort, c.MySQLDatabase)
}

// MigrationDSN returns the MySQL DSN with multiStatements enabled, used only during migrations.
func (c *Config) MigrationDSN() string {
	return fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?parseTime=true&multiStatements=true",
		c.MySQLUser, c.MySQLPassword, c.MySQLHost, c.MySQLPort, c.MySQLDatabase)
}

func requireEnv(key string, missing *[]string) string {
	v := os.Getenv(key)
	if v == "" {
		*missing = append(*missing, key)
	}
	return v
}

func envOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func parseIntEnv(key string, def int) (int, error) {
	v := os.Getenv(key)
	if v == "" {
		return def, nil
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def, fmt.Errorf("%s=%q is not a valid integer", key, v)
	}
	return n, nil
}

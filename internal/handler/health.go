package handler

import (
	"context"
	"database/sql"
	"net/http"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

type publicHealthResponse struct {
	Status string `json:"status"`
}

type detailHealthResponse struct {
	Status string `json:"status"`
	MySQL  string `json:"mysql"`
	Redis  string `json:"redis"`
}

type HealthHandler struct {
	db              *sql.DB
	redis           *goredis.Client
	healthSecretKey string
}

func NewHealthHandler(db *sql.DB, redis *goredis.Client) *HealthHandler {
	return &HealthHandler{db: db, redis: redis}
}

func NewHealthHandlerWithSecret(db *sql.DB, redis *goredis.Client, secretKey string) *HealthHandler {
	return &HealthHandler{db: db, redis: redis, healthSecretKey: secretKey}
}

// Handle serves GET /health — always public, returns minimal status.
func (h *HealthHandler) Handle(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), time.Second)
	defer cancel()

	mysqlOK := h.db.PingContext(ctx) == nil
	redisOK := h.redis.Ping(ctx).Err() == nil

	if mysqlOK && redisOK {
		WriteJSON(w, http.StatusOK, publicHealthResponse{Status: "ok"})
		return
	}
	WriteJSON(w, http.StatusServiceUnavailable, publicHealthResponse{Status: "degraded"})
}

// HandleDetails serves GET /health/details — protected by X-Health-Key header.
// If HealthSecretKey is empty, the endpoint returns 404 (disabled).
func (h *HealthHandler) HandleDetails(w http.ResponseWriter, r *http.Request) {
	if h.healthSecretKey == "" {
		WriteError(w, http.StatusNotFound, "not found", "ERR_NOT_FOUND")
		return
	}
	if r.Header.Get("X-Health-Key") != h.healthSecretKey {
		WriteError(w, http.StatusForbidden, "forbidden", "ERR_FORBIDDEN")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), time.Second)
	defer cancel()

	mysqlStatus := "up"
	if err := h.db.PingContext(ctx); err != nil {
		mysqlStatus = "down"
	}

	redisStatus := "up"
	if err := h.redis.Ping(ctx).Err(); err != nil {
		redisStatus = "down"
	}

	resp := detailHealthResponse{
		Status: "ok",
		MySQL:  mysqlStatus,
		Redis:  redisStatus,
	}
	statusCode := http.StatusOK
	if mysqlStatus == "down" || redisStatus == "down" {
		resp.Status = "degraded"
		statusCode = http.StatusServiceUnavailable
	}

	WriteJSON(w, statusCode, resp)
}

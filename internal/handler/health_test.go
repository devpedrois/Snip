package handler_test

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/alicebob/miniredis/v2"
	_ "github.com/go-sql-driver/mysql"
	goredis "github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/devpedrois/snip/internal/handler"
)

func openDeadMySQL(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("mysql", "root:@tcp(127.0.0.1:1)/nonexistent?timeout=500ms")
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() }) //nolint:errcheck,gosec
	return db
}

func startRedis(t *testing.T) *goredis.Client {
	t.Helper()
	mr := miniredis.RunT(t)
	return goredis.NewClient(&goredis.Options{Addr: mr.Addr()})
}

func deadRedis() *goredis.Client {
	return goredis.NewClient(&goredis.Options{Addr: "127.0.0.1:1"})
}

// TestHealthHandler_Public tests the public /health endpoint.
func TestHealthHandler_Public_OK(t *testing.T) {
	mr := miniredis.RunT(t)
	redis := goredis.NewClient(&goredis.Options{Addr: mr.Addr()})

	db, err := sql.Open("mysql", "root:@tcp(127.0.0.1:1)/nonexistent?timeout=500ms")
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() }) //nolint:errcheck,gosec

	h := handler.NewHealthHandler(db, redis)
	w := httptest.NewRecorder()
	h.Handle(w, httptest.NewRequest(http.MethodGet, "/health", nil))

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)

	var resp struct {
		Status string `json:"status"`
		MySQL  string `json:"mysql,omitempty"`
		Redis  string `json:"redis,omitempty"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "degraded", resp.Status)
	assert.Empty(t, resp.MySQL, "public endpoint must not expose infrastructure details")
	assert.Empty(t, resp.Redis, "public endpoint must not expose infrastructure details")
}

func TestHealthHandler_Public_Degraded(t *testing.T) {
	h := handler.NewHealthHandler(openDeadMySQL(t), deadRedis())
	w := httptest.NewRecorder()
	h.Handle(w, httptest.NewRequest(http.MethodGet, "/health", nil))

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)

	var resp struct{ Status string }
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "degraded", resp.Status)
}

// TestHealthHandler_Details tests the protected /health/details endpoint.
func TestHealthHandler_Details_NoHeader_Returns403(t *testing.T) {
	h := handler.NewHealthHandlerWithSecret(openDeadMySQL(t), startRedis(t), "secret123")
	w := httptest.NewRecorder()
	h.HandleDetails(w, httptest.NewRequest(http.MethodGet, "/health/details", nil))

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestHealthHandler_Details_WrongHeader_Returns403(t *testing.T) {
	h := handler.NewHealthHandlerWithSecret(openDeadMySQL(t), startRedis(t), "secret123")
	req := httptest.NewRequest(http.MethodGet, "/health/details", nil)
	req.Header.Set("X-Health-Key", "wrong")
	w := httptest.NewRecorder()
	h.HandleDetails(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestHealthHandler_Details_CorrectHeader_Returns200(t *testing.T) {
	mr := miniredis.RunT(t)
	redis := goredis.NewClient(&goredis.Options{Addr: mr.Addr()})

	db, err := sql.Open("mysql", "root:@tcp(127.0.0.1:1)/nonexistent?timeout=500ms")
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() }) //nolint:errcheck,gosec

	h := handler.NewHealthHandlerWithSecret(db, redis, "secret123")
	req := httptest.NewRequest(http.MethodGet, "/health/details", nil)
	req.Header.Set("X-Health-Key", "secret123")
	w := httptest.NewRecorder()
	h.HandleDetails(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)

	var resp struct {
		Status string `json:"status"`
		MySQL  string `json:"mysql"`
		Redis  string `json:"redis"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.NotEmpty(t, resp.MySQL)
	assert.NotEmpty(t, resp.Redis)
}

func TestHealthHandler_Details_EmptySecret_Returns404(t *testing.T) {
	h := handler.NewHealthHandlerWithSecret(openDeadMySQL(t), deadRedis(), "")
	req := httptest.NewRequest(http.MethodGet, "/health/details", nil)
	req.Header.Set("X-Health-Key", "anything")
	w := httptest.NewRecorder()
	h.HandleDetails(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

// Legacy tests kept for the public /health endpoint behaviour.
func TestHealthHandler_BothDown(t *testing.T) {
	h := handler.NewHealthHandler(openDeadMySQL(t), deadRedis())
	w := httptest.NewRecorder()
	h.Handle(w, httptest.NewRequest(http.MethodGet, "/health", nil))

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

func TestHealthHandler_RedisUpMySQLDown(t *testing.T) {
	h := handler.NewHealthHandler(openDeadMySQL(t), startRedis(t))
	w := httptest.NewRecorder()
	h.Handle(w, httptest.NewRequest(http.MethodGet, "/health", nil))

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

func TestHealthHandler_MySQLDownRedisDown(t *testing.T) {
	h := handler.NewHealthHandler(openDeadMySQL(t), deadRedis())
	w := httptest.NewRecorder()
	h.Handle(w, httptest.NewRequest(http.MethodGet, "/health", nil))

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

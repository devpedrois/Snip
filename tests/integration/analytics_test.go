//go:build integration

package integration

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAnalyticsClickCount(t *testing.T) {
	env := setupTestEnv(t)
	defer env.cleanup()

	body, _ := json.Marshal(map[string]string{"url": "https://analytics-test.example.com"})
	resp, err := http.Post(env.server.URL+"/api/shorten", "application/json", bytes.NewReader(body))
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusCreated, resp.StatusCode)

	var shortenResp struct {
		Hash string `json:"hash"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&shortenResp))

	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	for i := 0; i < 10; i++ {
		r, err := client.Get(env.server.URL + "/" + shortenResp.Hash)
		require.NoError(t, err)
		r.Body.Close()
		assert.Equal(t, http.StatusMovedPermanently, r.StatusCode)
	}

	// Wait for async workers to drain
	time.Sleep(2 * time.Second)

	statsResp, err := http.Get(env.server.URL + "/api/analytics/" + shortenResp.Hash)
	require.NoError(t, err)
	defer statsResp.Body.Close()
	require.Equal(t, http.StatusOK, statsResp.StatusCode)

	var stats struct {
		TotalClicks int64 `json:"total_clicks"`
	}
	require.NoError(t, json.NewDecoder(statsResp.Body).Decode(&stats))
	assert.GreaterOrEqual(t, stats.TotalClicks, int64(10))
}

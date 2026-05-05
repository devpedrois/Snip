//go:build integration

package integration

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestShortenAndRedirect(t *testing.T) {
	env := setupTestEnv(t)
	defer env.cleanup()

	longURL := "https://example.com/some/very/long/path"

	body, _ := json.Marshal(map[string]string{"url": longURL})
	resp, err := http.Post(env.server.URL+"/api/shorten", "application/json", bytes.NewReader(body))
	require.NoError(t, err)
	defer resp.Body.Close()

	require.Equal(t, http.StatusCreated, resp.StatusCode)

	var shortenResp struct {
		Hash     string `json:"hash"`
		ShortURL string `json:"short_url"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&shortenResp))
	assert.Len(t, shortenResp.Hash, 7)
	assert.NotEmpty(t, shortenResp.ShortURL)

	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	redirectResp, err := client.Get(env.server.URL + "/" + shortenResp.Hash)
	require.NoError(t, err)
	defer redirectResp.Body.Close()

	assert.Equal(t, http.StatusMovedPermanently, redirectResp.StatusCode)
	assert.Equal(t, longURL, redirectResp.Header.Get("Location"))
}

func TestRedirectNotFound(t *testing.T) {
	env := setupTestEnv(t)
	defer env.cleanup()

	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	resp, err := client.Get(env.server.URL + "/0000000")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestShortenInvalidURL(t *testing.T) {
	env := setupTestEnv(t)
	defer env.cleanup()

	body, _ := json.Marshal(map[string]string{"url": "not-a-url"})
	resp, err := http.Post(env.server.URL+"/api/shorten", "application/json", bytes.NewReader(body))
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

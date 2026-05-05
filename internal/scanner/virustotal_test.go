package scanner_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/devpedrois/snip/internal/scanner"
)

func analysisResponse(status string, malicious int) []byte {
	resp := map[string]any{
		"data": map[string]any{
			"attributes": map[string]any{
				"status": status,
				"stats": map[string]any{
					"harmless":   70,
					"malicious":  malicious,
					"suspicious": 2,
					"undetected": 10,
					"timeout":    3,
				},
			},
		},
	}
	b, _ := json.Marshal(resp)
	return b
}

func submitResponse(id string) []byte {
	resp := map[string]any{
		"data": map[string]any{
			"id":   id,
			"type": "analysis",
		},
	}
	b, _ := json.Marshal(resp)
	return b
}

func TestVirusTotalScanner_Clean(t *testing.T) {
	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if r.Method == http.MethodPost {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(submitResponse("analysis-id-123"))
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(analysisResponse("completed", 0))
	}))
	defer srv.Close()

	sc := newTestScanner(t, srv.URL, 1)
	result, err := sc.Scan(context.Background(), "https://example.com")

	require.NoError(t, err)
	assert.Equal(t, scanner.ScanClean, result.Status)
	assert.Equal(t, 0, result.Positives)
}

func TestVirusTotalScanner_Malicious(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(submitResponse("analysis-id-456"))
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(analysisResponse("completed", 5))
	}))
	defer srv.Close()

	sc := newTestScanner(t, srv.URL, 2)
	result, err := sc.Scan(context.Background(), "https://malware.example.com")

	require.NoError(t, err)
	assert.Equal(t, scanner.ScanMalicious, result.Status)
	assert.Equal(t, 5, result.Positives)
}

func TestVirusTotalScanner_Timeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(submitResponse("analysis-id-slow"))
			return
		}
		time.Sleep(10 * time.Second)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(analysisResponse("completed", 0))
	}))
	defer srv.Close()

	sc := newTestScanner(t, srv.URL, 2)
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	result, err := sc.Scan(ctx, "https://slow.example.com")

	require.NoError(t, err, "Scan must never return error")
	assert.Equal(t, scanner.ScanUnverified, result.Status)
}

func TestVirusTotalScanner_HTTP429(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()

	sc := newTestScanner(t, srv.URL, 2)
	result, err := sc.Scan(context.Background(), "https://example.com")

	require.NoError(t, err, "Scan must never return error")
	assert.Equal(t, scanner.ScanUnverified, result.Status)
}

func TestVirusTotalScanner_NetworkError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(submitResponse("analysis-id-net"))
	}))

	sc := newTestScanner(t, srv.URL, 2)
	srv.Close()

	result, err := sc.Scan(context.Background(), "https://example.com")

	require.NoError(t, err, "Scan must never return error")
	assert.Equal(t, scanner.ScanUnverified, result.Status)
}

func TestVirusTotalScanner_Polling(t *testing.T) {
	pollCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(submitResponse("analysis-id-poll"))
			return
		}
		pollCount++
		w.WriteHeader(http.StatusOK)
		if pollCount < 3 {
			_, _ = w.Write(analysisResponse("queued", 0))
		} else {
			_, _ = w.Write(analysisResponse("completed", 0))
		}
	}))
	defer srv.Close()

	sc := newTestScanner(t, srv.URL, 2)
	result, err := sc.Scan(context.Background(), "https://example.com")

	require.NoError(t, err)
	assert.Equal(t, scanner.ScanClean, result.Status)
	assert.GreaterOrEqual(t, pollCount, 3)
}

func newTestScanner(t *testing.T, baseURL string, minPositives int) *scanner.VirusTotalScanner {
	t.Helper()
	return scanner.NewVirusTotalScannerWithBase(baseURL, "test-api-key", 30*time.Second, minPositives)
}

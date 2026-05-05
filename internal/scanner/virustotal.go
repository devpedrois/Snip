package scanner

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	vtSubmitURL    = "https://www.virustotal.com/api/v3/urls"
	vtAnalysisURL  = "https://www.virustotal.com/api/v3/analyses/"
	vtGUIBase      = "https://www.virustotal.com/gui/url/"
	vtPollInterval = 2 * time.Second
)

type VirusTotalScanner struct {
	apiKey       string
	client       *http.Client
	minPositives int
	submitURL    string
	analysisURL  string
}

func NewVirusTotalScanner(apiKey string, timeout time.Duration, minPositives int) *VirusTotalScanner {
	return newScanner(apiKey, vtSubmitURL, vtAnalysisURL, timeout, minPositives)
}

// NewVirusTotalScannerWithBase creates a scanner pointing at a custom base URL (for testing).
func NewVirusTotalScannerWithBase(baseURL, apiKey string, timeout time.Duration, minPositives int) *VirusTotalScanner {
	return newScanner(apiKey, baseURL+"/api/v3/urls", baseURL+"/api/v3/analyses/", timeout, minPositives)
}

func newScanner(apiKey, submitURL, analysisURL string, timeout time.Duration, minPositives int) *VirusTotalScanner {
	client := &http.Client{
		Timeout: timeout,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	return &VirusTotalScanner{
		apiKey:       apiKey,
		client:       client,
		minPositives: minPositives,
		submitURL:    submitURL,
		analysisURL:  analysisURL,
	}
}

func (v *VirusTotalScanner) Scan(ctx context.Context, rawURL string) (ScanResult, error) {
	id, ok := v.submit(ctx, rawURL)
	if !ok {
		return ScanResult{Status: ScanUnverified, ScannedAt: time.Now()}, nil
	}

	result := v.poll(ctx, id, rawURL)
	return result, nil
}

func (v *VirusTotalScanner) submit(ctx context.Context, rawURL string) (string, bool) {
	body := url.Values{"url": []string{rawURL}}.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, v.submitURL, strings.NewReader(body))
	if err != nil {
		slog.Warn("virustotal: submit: build request", "err", err)
		return "", false
	}
	req.Header.Set("x-apikey", v.apiKey)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := v.client.Do(req)
	if err != nil {
		slog.Warn("virustotal: submit: request", "err", err)
		return "", false
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		slog.Warn("virustotal: submit: unexpected status", "status", resp.StatusCode)
		return "", false
	}

	var parsed struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		slog.Warn("virustotal: submit: decode response", "err", err)
		return "", false
	}

	if parsed.Data.ID == "" {
		slog.Warn("virustotal: submit: empty analysis id")
		return "", false
	}

	return parsed.Data.ID, true
}

func (v *VirusTotalScanner) poll(ctx context.Context, id, rawURL string) ScanResult {
	permalink := vtGUIBase + base64.RawURLEncoding.EncodeToString([]byte(rawURL)) + "/detection"

	for {
		select {
		case <-ctx.Done():
			return ScanResult{Status: ScanUnverified, ScannedAt: time.Now()}
		case <-time.After(vtPollInterval):
		}

		result, done := v.fetchAnalysis(ctx, id, permalink)
		if done {
			return result
		}
	}
}

func (v *VirusTotalScanner) fetchAnalysis(ctx context.Context, id, permalink string) (ScanResult, bool) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, v.analysisURL+id, nil)
	if err != nil {
		slog.Warn("virustotal: poll: build request", "err", err)
		return ScanResult{Status: ScanUnverified, ScannedAt: time.Now()}, true
	}
	req.Header.Set("x-apikey", v.apiKey)

	resp, err := v.client.Do(req)
	if err != nil {
		slog.Warn("virustotal: poll: request", "err", err)
		return ScanResult{Status: ScanUnverified, ScannedAt: time.Now()}, true
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		slog.Warn("virustotal: poll: unexpected status", "status", resp.StatusCode)
		return ScanResult{Status: ScanUnverified, ScannedAt: time.Now()}, true
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		slog.Warn("virustotal: poll: read body", "err", err)
		return ScanResult{Status: ScanUnverified, ScannedAt: time.Now()}, true
	}

	var parsed struct {
		Data struct {
			Attributes struct {
				Status string `json:"status"`
				Stats  struct {
					Harmless   int `json:"harmless"`
					Malicious  int `json:"malicious"`
					Suspicious int `json:"suspicious"`
					Undetected int `json:"undetected"`
					Timeout    int `json:"timeout"`
				} `json:"stats"`
			} `json:"attributes"`
		} `json:"data"`
	}
	if err := json.Unmarshal(data, &parsed); err != nil {
		slog.Warn("virustotal: poll: decode response", "err", err)
		return ScanResult{Status: ScanUnverified, ScannedAt: time.Now()}, true
	}

	if parsed.Data.Attributes.Status != "completed" {
		return ScanResult{}, false
	}

	s := parsed.Data.Attributes.Stats
	total := s.Harmless + s.Malicious + s.Suspicious + s.Undetected + s.Timeout

	status := ScanClean
	if s.Malicious >= v.minPositives {
		status = ScanMalicious
	}

	return ScanResult{
		Status:    status,
		Positives: s.Malicious,
		Total:     total,
		ScannedAt: time.Now(),
		Permalink: permalink,
	}, true
}

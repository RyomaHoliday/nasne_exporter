package nasne

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"
)

// Client accesses nasne HTTP APIs and extracts a stable metrics snapshot.
type Client struct {
	baseURL    *url.URL
	httpClient *http.Client
	endpoints  []string
}

// Snapshot is a normalized view used by the exporter.
type Snapshot struct {
	Name                   string
	ProductName            string
	HardwareVersion        string
	SoftwareVersion        string
	HDDSizeBytes           float64
	HDDUsageBytes          float64
	DTCPIPClients          float64
	Recordings             float64
	RecordedTitles         float64
	ReservedTitles         float64
	ReservedConflictTitles float64
	ReservedNotFoundTitles float64
}

func NewClient(rawBaseURL string, endpoints []string, timeout time.Duration) (*Client, error) {
	if rawBaseURL == "" {
		return nil, fmt.Errorf("base URL is required")
	}

	u, err := url.Parse(rawBaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse base URL: %w", err)
	}
	if u.Scheme == "" || u.Host == "" {
		return nil, fmt.Errorf("base URL must include scheme and host")
	}

	if len(endpoints) == 0 {
		endpoints = []string{"/status", "/storage", "/schedule"}
	}

	normalized := make([]string, 0, len(endpoints))
	for _, ep := range endpoints {
		ep = strings.TrimSpace(ep)
		if ep == "" {
			continue
		}
		if !strings.HasPrefix(ep, "/") {
			ep = "/" + ep
		}
		normalized = append(normalized, ep)
	}
	if len(normalized) == 0 {
		normalized = []string{"/status", "/storage", "/schedule"}
	}

	return &Client{
		baseURL: u,
		httpClient: &http.Client{
			Timeout: timeout,
		},
		endpoints: normalized,
	}, nil
}

func (c *Client) FetchSnapshot(ctx context.Context) (Snapshot, error) {
	payload := map[string]any{}
	var lastErr error

	for _, ep := range c.endpoints {
		data, err := c.getJSON(ctx, ep)
		if err != nil {
			lastErr = err
			continue
		}
		payload[path.Base(ep)] = data
	}

	if len(payload) == 0 {
		if lastErr == nil {
			lastErr = fmt.Errorf("no endpoints configured")
		}
		return Snapshot{}, fmt.Errorf("fetch snapshot: %w", lastErr)
	}

	return ExtractSnapshot(payload), nil
}

func (c *Client) getJSON(ctx context.Context, endpoint string) (map[string]any, error) {
	u := *c.baseURL
	u.Path = path.Join(c.baseURL.Path, endpoint)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("create request %q: %w", endpoint, err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request %q: %w", endpoint, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return nil, fmt.Errorf("request %q: status=%d body=%q", endpoint, resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var data map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, fmt.Errorf("decode %q: %w", endpoint, err)
	}

	return data, nil
}

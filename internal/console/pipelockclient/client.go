// Copyright 2026 Josh Waldrep
// SPDX-License-Identifier: Apache-2.0

// Package pipelockclient is a typed HTTP client for pipelock's runtime API.
package pipelockclient

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// Options configures a Client.
type Options struct {
	BaseURL       string
	KillswitchURL string
	APIToken      string
	HTTP          *http.Client
}

// Client talks to a pipelock instance's HTTP API.
type Client struct {
	baseURL       string
	killswitchURL string
	apiToken      string
	http          *http.Client
}

// New constructs a Client, applying sensible defaults.
func New(o Options) *Client {
	if o.HTTP == nil {
		o.HTTP = &http.Client{Timeout: 5 * time.Second}
	}
	if o.KillswitchURL == "" {
		o.KillswitchURL = o.BaseURL
	}
	return &Client{baseURL: o.BaseURL, killswitchURL: o.KillswitchURL, apiToken: o.APIToken, http: o.HTTP}
}

// Stats mirrors pipelock's /stats JSON (subset the console renders).
type Stats struct {
	UptimeSeconds float64 `json:"uptime_seconds"`
	Requests      struct {
		Total     int64   `json:"total"`
		Allowed   int64   `json:"allowed"`
		Blocked   int64   `json:"blocked"`
		BlockRate float64 `json:"block_rate"`
	} `json:"requests"`
	Tunnels           int64 `json:"tunnels"`
	WebSockets        int64 `json:"websockets"`
	TopBlockedDomains []struct {
		Name  string `json:"name"`
		Count int64  `json:"count"`
	} `json:"top_blocked_domains"`
	TopScanners []struct {
		Name  string `json:"name"`
		Count int64  `json:"count"`
	} `json:"top_scanners"`
	Sessions struct {
		Active      int64 `json:"active"`
		Anomalies   int64 `json:"anomalies"`
		Escalations int64 `json:"escalations"`
	} `json:"sessions"`
}

func (c *Client) getJSON(ctx context.Context, url string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	// /api/v1/sessions requires auth; /stats and /health ignore the header,
	// so attaching the token unconditionally is safe.
	if c.apiToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiToken)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("request %s: %w", url, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%s returned %d", url, resp.StatusCode)
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

// GetStats fetches /stats.
func (c *Client) GetStats(ctx context.Context) (*Stats, error) {
	var s Stats
	if err := c.getJSON(ctx, c.baseURL+"/stats", &s); err != nil {
		return nil, err
	}
	return &s, nil
}

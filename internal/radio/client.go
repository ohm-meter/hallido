// Package radio kapselt die Radio-Browser-API (ADR 0011).
package radio

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"sonos-radio-agent/internal/apperr"
)

const (
	defaultTimeout   = 4 * time.Second // je Mirror
	defaultUserAgent = "sonos-radio-agent/1.0 (local Sonos scene controller)"
	maxResponseBytes = 512 * 1024
)

// defaultBaseURLs sind Radio-Browser-Mirror; bei Ausfall des ersten wird der naechste versucht.
var defaultBaseURLs = []string{
	"https://de1.api.radio-browser.info",
	"https://de2.api.radio-browser.info",
	"https://fi1.api.radio-browser.info",
}

type Station struct {
	Name    string `json:"name"`
	URL     string `json:"url"`
	Country string `json:"country"`
	Tags    string `json:"tags,omitempty"`
}

type apiStation struct {
	Name    string `json:"name"`
	URL     string `json:"url"`
	Country string `json:"country"`
	Tags    string `json:"tags"`
}

type Client struct {
	baseURLs   []string
	httpClient *http.Client
	userAgent  string
}

func NewClient() *Client {
	return &Client{
		baseURLs:   append([]string(nil), defaultBaseURLs...),
		httpClient: &http.Client{Timeout: defaultTimeout},
		userAgent:  defaultUserAgent,
	}
}

func (c *Client) WithBaseURL(base string) *Client {
	if strings.TrimSpace(base) != "" {
		c.baseURLs = []string{strings.TrimRight(strings.TrimSpace(base), "/")}
	}
	return c
}

func (c *Client) WithHTTPClient(hc *http.Client) *Client {
	if hc != nil {
		c.httpClient = hc
	}
	return c
}

func (c *Client) Search(ctx context.Context, query string, limit int) ([]Station, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, apperr.New(apperr.CodeBadRequest, "Search term missing")
	}
	if limit <= 0 {
		limit = 10
	}
	if limit > 50 {
		limit = 50
	}
	var body []byte
	var lastErr *apperr.Error
	for _, base := range c.baseURLs {
		b, aerr, retry := c.fetch(ctx, base, query, limit)
		if aerr == nil {
			body = b
			lastErr = nil
			break
		}
		lastErr = aerr
		if !retry || ctx.Err() != nil {
			break
		}
	}
	if lastErr != nil {
		return nil, lastErr
	}
	var raw []apiStation
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, apperr.New(apperr.CodeRadioSearchFailed, "Invalid radio response").WithHint("Unexpected JSON from Radio Browser.")
	}
	out := make([]Station, 0, len(raw))
	for _, st := range raw {
		name := strings.TrimSpace(st.Name)
		streamURL := strings.TrimSpace(st.URL)
		if name == "" || streamURL == "" {
			continue
		}
		out = append(out, Station{Name: name, URL: streamURL, Country: strings.TrimSpace(st.Country), Tags: strings.TrimSpace(st.Tags)})
	}
	return out, nil
}

// fetch fragt einen Mirror ab. retry=true bei Netz-/5xx-Fehlern (naechster Mirror sinnvoll).
func (c *Client) fetch(ctx context.Context, base, query string, limit int) (body []byte, aerr *apperr.Error, retry bool) {
	endpoint := fmt.Sprintf("%s/json/stations/search?name=%s&limit=%d", base, url.QueryEscape(query), limit)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, apperr.New(apperr.CodeRadioSearchFailed, "Radio search could not be started").WithHint(err.Error()), false
	}
	req.Header.Set("User-Agent", c.userAgent)
	req.Header.Set("Accept", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, apperr.New(apperr.CodeRadioSearchFailed, "Radio Browser unreachable").
			WithHint("Check internet connection and DNS."), true
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, apperr.New(apperr.CodeRadioSearchFailed,
				fmt.Sprintf("Radio Browser responded with HTTP %d", resp.StatusCode)).WithHint("Try again later."),
			resp.StatusCode >= 500
	}
	body, err = io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	if err != nil {
		return nil, apperr.New(apperr.CodeRadioSearchFailed, "Incomplete radio response").WithHint(err.Error()), true
	}
	return body, nil, false
}

package external

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"mikan/internal/panel/external/parsers"
)

const (
	// FetchTimeout is the timeout for fetching an upstream subscription.
	FetchTimeout = 30 * time.Second

	// DefaultInterval is the default interval for fetching external subscriptions (1 hour).
	DefaultInterval = 1 * time.Hour

	// MinInterval is the minimum allowed interval (5 minutes).
	MinInterval = 5 * time.Minute
)

// Server represents one server from an upstream subscription.
// Server represents one server from an upstream subscription.
type Server = parsers.Server

// Fetcher fetches and parses external subscriptions.
type Fetcher struct {
	client *http.Client
}

func NewFetcher(timeout time.Duration) *Fetcher {
	return &Fetcher{
		client: &http.Client{
			Timeout: timeout,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if len(via) >= 10 {
					return http.ErrUseLastResponse
				}
				return nil
			},
		},
	}
}

type FetchResult struct {
	Servers []parsers.Server
}

// Fetch downloads and parses an external subscription URL.
func (f *Fetcher) Fetch(ctx context.Context, urlStr string) (*FetchResult, error) {
	ctx, cancel := context.WithTimeout(ctx, FetchTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", urlStr, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("User-Agent", "Hikari/1.0 (+https://github.com/ShelbyHell/Hikari)")

	resp, err := f.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}

	servers, err := parsers.ParseSubscription(body, urlStr)
	if err != nil {
		return nil, fmt.Errorf("parse: %w", err)
	}

	return &FetchResult{Servers: servers}, nil
}

// MihomoJSON is a server's mihomo map as the store keeps it.
func MihomoJSON(s Server) string {
	b, _ := json.Marshal(s.Mihomo)
	return string(b)
}

// Rows converts servers to the database row format.
func Rows(externalSubID int64, servers []Server) []ExternalServerRow {
	rows := make([]ExternalServerRow, 0, len(servers))
	for _, s := range servers {
		rows = append(rows, ExternalServerRow{
			ExternalSubID: externalSubID,
			Name:          s.Name,
			Link:          s.Link,
			Enabled:       s.Enabled,
			Mihomo:        MihomoJSON(s),
			CreatedAt:     time.Now().Unix(),
		})
	}
	return rows
}

// ExternalServerRow is the database row for an external server.
type ExternalServerRow struct {
	ExternalSubID int64
	Name          string
	Link          string
	Enabled       bool
	Mihomo        string
	CreatedAt     int64
}

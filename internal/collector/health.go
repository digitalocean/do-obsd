package collector

import (
	"context"
	"fmt"
	"io"
	"net/http"
)

// httpClient abstracts HTTP requests for testability.
type httpClient interface {
	Do(req *http.Request) (*http.Response, error)
}

// CheckHealth probes the collector's healthcheck extension endpoint.
// Returns nil if the collector responds with HTTP 200, an error otherwise.
func (c *Collector) CheckHealth(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, HealthTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.healthURL, nil)
	if err != nil {
		return fmt.Errorf("create health request: %w", err)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("health check request: %w", err)
	}
	// Drain body before close to allow TCP connection reuse across the
	// repeated health checks (runs every 15s for the lifetime of the process).
	defer func() {
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("health check returned status %d", resp.StatusCode)
	}

	return nil
}

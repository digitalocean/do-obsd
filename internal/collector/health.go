package collector

import (
	"context"
	"fmt"
	"net/http"
)

// httpClient abstracts HTTP requests for testability.
type httpClient interface {
	Do(req *http.Request) (*http.Response, error)
}

// healthEndpointOverride allows tests to redirect health checks to a test server.
var healthEndpointOverride string

func healthEndpoint() string {
	if healthEndpointOverride != "" {
		return healthEndpointOverride
	}
	return HealthEndpoint
}

// CheckHealth probes the collector's healthcheck extension endpoint.
// Returns nil if the collector responds with HTTP 200, an error otherwise.
func (c *Collector) CheckHealth(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, HealthTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, healthEndpoint(), nil)
	if err != nil {
		return fmt.Errorf("create health request: %w", err)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("health check request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("health check returned status %d", resp.StatusCode)
	}

	return nil
}

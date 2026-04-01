package collector

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"time"
)

const (
	defaultUserAgent        = "do-obsd"
	installOperationTimeout = 30 * time.Minute
	defaultDownloadAttempts = 5
	drainBodyLimit          = 64 << 10
)

func (c *Collector) fetchArtifact(ctx context.Context) (*http.Response, error) {
	attempts := defaultDownloadAttempts
	if c.downloadMaxAttempts > 0 {
		attempts = c.downloadMaxAttempts
	}

	for attempt := 1; attempt <= attempts; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, CollectorBinaryURL, nil)
		if err != nil {
			return nil, fmt.Errorf("build download request: %w", err)
		}

		ua := defaultUserAgent
		if c.userAgent != "" {
			ua = c.userAgent
		}
		req.Header.Set("User-Agent", ua)
		req.Header.Set("Accept", "*/*")

		resp, err := c.http.Do(req)
		if err != nil {
			if !retryableRequestErr(err) || attempt >= attempts {
				return nil, fmt.Errorf("download collector: %w", err)
			}
			slog.Warn("collector download failed, retrying", "attempt", attempt, "attempts", attempts, "err", err)
			if waitOrCancel(ctx, retryBackoff(attempt)) != nil {
				return nil, fmt.Errorf("download collector: %w", ctx.Err())
			}
			continue
		}

		if resp.StatusCode == http.StatusOK {
			return resp, nil
		}

		drainAndClose(resp)
		if !statusRetryable(resp.StatusCode) || attempt >= attempts {
			return nil, fmt.Errorf("download collector: HTTP %d %s", resp.StatusCode, http.StatusText(resp.StatusCode))
		}
		slog.Warn("collector download returned non-OK, retrying", "attempt", attempt, "attempts", attempts, "status", resp.StatusCode)
		if waitOrCancel(ctx, retryBackoff(attempt)) != nil {
			return nil, fmt.Errorf("download collector: %w", ctx.Err())
		}
	}

	return nil, fmt.Errorf("download collector: exhausted retries")
}

func waitOrCancel(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func retryBackoff(attempt int) time.Duration {
	shift := attempt - 1
	if shift > 5 {
		shift = 5
	}
	d := time.Second * time.Duration(1<<uint(shift))
	if d > 32*time.Second {
		d = 32 * time.Second
	}
	return d
}

func statusRetryable(code int) bool {
	switch code {
	case http.StatusTooManyRequests, http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		return true
	default:
		return false
	}
}

func retryableRequestErr(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	var certErr *tls.CertificateVerificationError
	return !errors.As(err, &certErr)
}

func drainAndClose(resp *http.Response) {
	if resp == nil || resp.Body == nil {
		return
	}
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, drainBodyLimit))
	_ = resp.Body.Close()
}

func validateArtifactURL() {
	u, err := url.Parse(CollectorBinaryURL)
	if err != nil || u.Scheme != "https" || u.Host == "" {
		panic("collector: CollectorBinaryURL must be a valid https URL with a host")
	}
}

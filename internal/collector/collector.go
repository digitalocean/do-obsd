package collector

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

const (
	// CollectorBinaryURL is the HTTPS URL of the do-otelcol binary artifact.
	CollectorBinaryURL = "https://marlin.nyc3.cdn.digitaloceanspaces.com/do-otelcol/linux-amd64/do-otelcol"

	CollectorBin     = "/opt/digitalocean/bin/do-otelcol"
	CollectorService = "do-otelcol.service"

	installDownloadTimeout = 30 * time.Minute
)

func init() {
	u, err := url.Parse(CollectorBinaryURL)
	if err != nil || u.Scheme != "https" || u.Host == "" {
		panic("collector: CollectorBinaryURL must be a valid https URL with a host")
	}
}

//go:generate go tool mockgen -source=collector.go -package=collector -destination=mocks_test.go

// httpDoer is implemented by *http.Client for production.
type httpDoer interface {
	Do(req *http.Request) (*http.Response, error)
}

// osOperator abstracts OS file operations for testability.
type osOperator interface {
	CreateTemp(dir, pattern string) (tempFile, error)
	Remove(name string) error
	Rename(oldpath, newpath string) error
}

// tempFile abstracts *os.File for testability.
type tempFile interface {
	Name() string
	Write(b []byte) (n int, err error)
	Chmod(mode os.FileMode) error
	Close() error
}

// cmdRunner abstracts exec.Command for testability.
type cmdRunner interface {
	Run(name string, args ...string) ([]byte, error)
}

// Collector manages the do-otelcol lifecycle.
type Collector struct {
	os   osOperator
	cmd  cmdRunner
	http httpDoer
}

// New returns a Collector with real OS, HTTP, and exec implementations.
func New() *Collector {
	return &Collector{
		os:  &realOSOperator{},
		cmd: &realCmdRunner{},
		http: &http.Client{
			Timeout: installDownloadTimeout,
		},
	}
}

// Install downloads the collector binary from CollectorBinaryURL and installs it atomically at CollectorBin.
func (c *Collector) Install() error {
	start := time.Now()
	slog.Info("installing collector", "url", CollectorBinaryURL, "dst", CollectorBin)

	ctx, cancel := context.WithTimeout(context.Background(), installDownloadTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, CollectorBinaryURL, nil)
	if err != nil {
		return fmt.Errorf("build download request: %w", err)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("download collector: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download collector: HTTP %d %s", resp.StatusCode, http.StatusText(resp.StatusCode))
	}

	tmp, err := c.os.CreateTemp(filepath.Dir(CollectorBin), ".do-otelcol-*")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpPath := tmp.Name()
	defer c.os.Remove(tmpPath) //nolint:errcheck

	if _, err := io.Copy(tmp, resp.Body); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write downloaded binary: %w", err)
	}

	if err := tmp.Chmod(0o755); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("chmod binary: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp file: %w", err)
	}

	if err := c.os.Rename(tmpPath, CollectorBin); err != nil {
		return fmt.Errorf("install binary: %w", err)
	}
	slog.Info("collector installed", "dst", CollectorBin, "duration", time.Since(start).Round(time.Millisecond))
	return nil
}

// Start starts do-otelcol.service via systemctl.
func (c *Collector) Start() error {
	slog.Info("starting collector", "service", CollectorService)
	out, err := c.cmd.Run("systemctl", "start", CollectorService)
	if err != nil {
		return fmt.Errorf("systemctl start %s: %w (output: %s)", CollectorService, err, out)
	}
	return nil
}

// Stop stops do-otelcol.service via systemctl.
func (c *Collector) Stop() error {
	slog.Info("stopping collector", "service", CollectorService)
	out, err := c.cmd.Run("systemctl", "stop", CollectorService)
	if err != nil {
		return fmt.Errorf("systemctl stop %s: %w (output: %s)", CollectorService, err, out)
	}
	return nil
}

// realOSOperator is the production implementation of osOperator.
type realOSOperator struct{}

func (r *realOSOperator) CreateTemp(dir, pattern string) (tempFile, error) {
	return os.CreateTemp(dir, pattern)
}

func (r *realOSOperator) Remove(name string) error {
	return os.Remove(name)
}

func (r *realOSOperator) Rename(oldpath, newpath string) error {
	return os.Rename(oldpath, newpath)
}

// realCmdRunner is the production implementation of cmdRunner.
type realCmdRunner struct{}

func (r *realCmdRunner) Run(name string, args ...string) ([]byte, error) {
	return exec.Command(name, args...).CombinedOutput() //nolint:gosec
}

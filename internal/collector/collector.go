package collector

import (
	"fmt"
	"io"
	"log/slog"
	"path/filepath"
)

const (
	// bundlePath is where the package ships the collector binary.
	bundlePath = "/opt/digitalocean/bundle/do-otelcol"

	collectorBin        = "/opt/digitalocean/bin/do-otelcol"
	collectorConfigPath = "/etc/do-otelcol/config.yaml"
)

// Collector manages the do-otelcol binary and configuration.
type Collector struct {
	os osOperator
}

// New returns a Collector with real OS implementations.
func New() *Collector {
	return &Collector{
		os: &realOSOperator{},
	}
}

// Install copies the bundled binary from bundlePath to collectorBin atomically.
// The bundle is shipped with the package; the binary is placed once at startup so
// that the systemd-managed do-otelcol.service can exec it.
func (c *Collector) Install() error {
	slog.Info("installing collector", "src", bundlePath, "dst", collectorBin)

	src, err := c.os.Open(bundlePath)
	if err != nil {
		return fmt.Errorf("open bundle: %w", err)
	}
	defer func() { _ = src.Close() }()

	tmp, err := c.os.CreateTemp(filepath.Dir(collectorBin), ".do-otelcol-*")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpPath := tmp.Name()
	defer c.os.Remove(tmpPath) //nolint:errcheck

	if _, err := io.Copy(tmp, src); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("copy binary: %w", err)
	}
	if err := tmp.Chmod(0755); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("chmod binary: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp file: %w", err)
	}
	if err := c.os.Rename(tmpPath, collectorBin); err != nil {
		return fmt.Errorf("install binary: %w", err)
	}
	return nil
}

// WriteConfig atomically writes data to collectorConfigPath.
// The temp-file-then-rename sequence guarantees do-otelcol's fsnotify watcher
// never observes a partial write: the file either has the old content or the new.
func (c *Collector) WriteConfig(data []byte) error {
	slog.Info("writing collector config", "path", collectorConfigPath)

	tmp, err := c.os.CreateTemp(filepath.Dir(collectorConfigPath), ".config-*.yaml")
	if err != nil {
		return fmt.Errorf("create temp: %w", err)
	}
	tmpPath := tmp.Name()
	defer c.os.Remove(tmpPath) //nolint:errcheck

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write config: %w", err)
	}
	if err := tmp.Chmod(0640); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("chmod config: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp: %w", err)
	}
	if err := c.os.Rename(tmpPath, collectorConfigPath); err != nil {
		return fmt.Errorf("install config: %w", err)
	}
	return nil
}

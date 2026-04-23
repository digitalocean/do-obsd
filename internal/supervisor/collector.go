package supervisor

import (
	"fmt"
	"log/slog"
	"path/filepath"
)

const (
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

// WriteConfig atomically writes data to collectorConfigPath.
// The temp-file-then-rename sequence ensures do-otelcol never reads a partial write.
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

package collector

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

// DefaultCollectorConfigPath is used when DO_OBSD_COLLECTOR_CONFIG_PATH is unset.
const DefaultCollectorConfigPath = "/etc/do-otelcol/config.yaml"

// ResolvedCollectorConfigPath returns the path WriteConfig uses.
// Override with DO_OBSD_COLLECTOR_CONFIG_PATH for prototyping (e.g. under $HOME).
func ResolvedCollectorConfigPath() string {
	if p := strings.TrimSpace(os.Getenv("DO_OBSD_COLLECTOR_CONFIG_PATH")); p != "" {
		return p
	}
	return DefaultCollectorConfigPath
}

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

// WriteConfig atomically writes data to the resolved collector config path.
// The temp-file-then-rename sequence ensures do-otelcol never reads a partial write.
func (c *Collector) WriteConfig(data []byte) error {
	path := ResolvedCollectorConfigPath()
	slog.Info("writing collector config", "path", path)

	if err := os.MkdirAll(filepath.Dir(path), 0750); err != nil {
		return fmt.Errorf("ensure config dir: %w", err)
	}

	tmp, err := c.os.CreateTemp(filepath.Dir(path), ".config-*.yaml")
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
	if err := c.os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("install config: %w", err)
	}
	return nil
}

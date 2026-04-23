package supervisor

import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"

	"github.com/digitalocean/do-obsd/internal/vpcendpoint"
)

const (
	collectorConfigPath = "/etc/do-otelcol/config.yaml"
)

// Supervisor manages the do-otelcol binary and configuration.
type Supervisor struct {
	os osOperator
}

// New returns a Supervisor with real OS implementations.
func New() *Supervisor {
	return &Supervisor{
		os: &realOSOperator{},
	}
}

// Run discovers the VPC endpoint, writes the collector config, and blocks until ctx is cancelled.
func (s *Supervisor) Run(ctx context.Context) error {
	ip, err := vpcendpoint.New().Discover()
	if err != nil {
		return fmt.Errorf("discover vpc endpoint: %w", err)
	}
	slog.Info("discovered vpc endpoint", "ip", ip)

	config, err := BuildConfig(ip)
	if err != nil {
		return fmt.Errorf("build collector config: %w", err)
	}

	if err := s.writeConfig(config); err != nil {
		return fmt.Errorf("write collector config: %w", err)
	}

	<-ctx.Done()
	return nil
}

// writeConfig atomically writes data to collectorConfigPath.
// The temp-file-then-rename sequence ensures do-otelcol never reads a partial write.
func (s *Supervisor) writeConfig(data []byte) error {
	slog.Info("writing collector config", "path", collectorConfigPath)

	tmp, err := s.os.CreateTemp(filepath.Dir(collectorConfigPath), ".config-*.yaml")
	if err != nil {
		return fmt.Errorf("create temp: %w", err)
	}
	tmpPath := tmp.Name()
	defer s.os.Remove(tmpPath) //nolint:errcheck

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
	if err := s.os.Rename(tmpPath, collectorConfigPath); err != nil {
		return fmt.Errorf("install config: %w", err)
	}
	return nil
}

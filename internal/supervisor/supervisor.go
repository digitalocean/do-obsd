package supervisor

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/digitalocean/do-obsd/internal/vpcendpoint"
)

// Supervisor manages the do-otelcol binary and configuration.
type Supervisor struct {
	dropletType string
	os          osOperator
}

// New returns a Supervisor with real OS implementations.
func New(dropletType string) *Supervisor {
	return &Supervisor{
		dropletType: dropletType,
		os:          &realOSOperator{},
	}
}

// Run discovers the VPC endpoint, writes the collector config, and blocks until ctx is cancelled.
func (s *Supervisor) Run(ctx context.Context) error {
	ip, err := vpcendpoint.New().Discover()
	if err != nil {
		return fmt.Errorf("discover vpc endpoint: %w", err)
	}
	slog.Info("discovered vpc endpoint", "ip", ip)

	config, err := buildCollectorConfig(ip)
	if err != nil {
		return fmt.Errorf("build collector config: %w", err)
	}

	if err := s.writeCollectorConfig(config); err != nil {
		return fmt.Errorf("write collector config: %w", err)
	}

	<-ctx.Done()
	return nil
}

package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/digitalocean/do-obsd/internal/collector"
)

// Supervision loop parameters. These work together: the loop polls health every
// healthCheckInterval, and after failureThreshold consecutive failures it restarts
// do-otelcol with exponential backoff. restartCount resets only when the collector
// reports healthy again; it is NOT decremented over time.
const (
	healthCheckInterval = 15 * time.Second
	failureThreshold    = 3
	maxRestarts         = 5
	initialBackoff      = 5 * time.Second
	maxBackoff          = 60 * time.Second
)

func main() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, nil)))

	if err := run(); err != nil {
		slog.Error("fatal", "err", err)
		os.Exit(1)
	}
}

func run() error {
	slog.Info("starting")

	col := collector.New()

	if err := col.Install(); err != nil {
		return fmt.Errorf("install collector: %w", err)
	}

	if err := col.Start(); err != nil {
		return fmt.Errorf("start collector: %w", err)
	}

	return supervise(col)
}

// supervise polls the collector's healthcheck extension and restarts do-otelcol
// when it becomes unresponsive. This catches "running but broken" states (e.g.
// frozen process, resource starvation) that systemd's Restart=on-failure cannot
// detect. Will be replaced by OpAMP's push-based health reporting in the future.
func supervise(col *collector.Collector) error {
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGTERM, syscall.SIGINT)

	ticker := time.NewTicker(healthCheckInterval)
	defer ticker.Stop()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	consecutiveFails := 0
	restartCount := 0
	backoff := initialBackoff

	// shutdown cancels in-flight health checks and stops the collector.
	// Returns nil so the process exits cleanly with code 0.
	shutdown := func(sig os.Signal) error {
		slog.Info("received signal, stopping", "signal", sig)
		cancel()
		if err := col.Stop(); err != nil {
			slog.Warn("stop collector failed", "err", err)
		}
		return nil
	}

	for {
		select {
		case <-ticker.C:
			if err := col.CheckHealth(ctx); err != nil {
				consecutiveFails++
				slog.Warn("health check failed",
					"err", err,
					"consecutive_failures", consecutiveFails,
					"threshold", failureThreshold,
				)

				if consecutiveFails >= failureThreshold {
					if restartCount >= maxRestarts {
						slog.Error("max restarts reached, not restarting collector",
							"restart_count", restartCount,
						)
						// Reset so we don't log this every tick; counters
						// fully reset if the collector becomes healthy again.
						consecutiveFails = 0
						continue
					}

					slog.Warn("restarting collector",
						"backoff", backoff,
						"restart_count", restartCount+1,
						"max_restarts", maxRestarts,
					)

					// select instead of time.Sleep so SIGTERM during backoff
					// is handled immediately rather than blocking up to maxBackoff.
					select {
					case <-time.After(backoff):
					case sig := <-sigs:
						return shutdown(sig)
					}

					// Only count successful restarts against the budget; a failed
					// systemctl call (e.g. permission denied) should not exhaust
					// our restart attempts since the collector was never restarted.
					if err := col.Restart(); err != nil {
						slog.Error("restart collector failed", "err", err)
					} else {
						restartCount++
					}
					backoff = min(backoff*2, maxBackoff)
					consecutiveFails = 0
				}
			} else {
				if consecutiveFails > 0 || restartCount > 0 {
					slog.Info("collector healthy, resetting counters",
						"previous_failures", consecutiveFails,
						"previous_restarts", restartCount,
					)
				}
				consecutiveFails = 0
				restartCount = 0
				backoff = initialBackoff
			}

		case sig := <-sigs:
			return shutdown(sig)
		}
	}
}

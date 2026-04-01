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

const (
	healthCheckInterval  = 15 * time.Second
	failureThreshold     = 3
	maxRestarts          = 5
	initialBackoff       = 5 * time.Second
	maxBackoff           = 60 * time.Second
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

func supervise(col *collector.Collector) error {
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGTERM, syscall.SIGINT)

	ticker := time.NewTicker(healthCheckInterval)
	defer ticker.Stop()

	ctx := context.Background()
	consecutiveFails := 0
	restartCount := 0
	backoff := initialBackoff

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
						continue
					}

					slog.Warn("restarting collector",
						"backoff", backoff,
						"restart_count", restartCount+1,
						"max_restarts", maxRestarts,
					)
					time.Sleep(backoff)

					if err := col.Restart(); err != nil {
						slog.Error("restart collector failed", "err", err)
					}

					restartCount++
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
			slog.Info("received signal, stopping", "signal", sig)
			if err := col.Stop(); err != nil {
				slog.Warn("stop collector failed", "err", err)
			}
			return nil
		}
	}
}

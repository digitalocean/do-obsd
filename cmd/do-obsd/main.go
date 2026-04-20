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
	"github.com/digitalocean/do-obsd/internal/opamp"
)

var version = "dev"

func main() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, nil)))

	if err := run(); err != nil {
		slog.Error("fatal", "err", err)
		os.Exit(1)
	}
}

func run() error {
	slog.Info("starting", "version", version, "collector_config_path", collector.ResolvedCollectorConfigPath())

	col := collector.New()

	if err := col.WriteConfig(collector.GPUConfig); err != nil {
		return fmt.Errorf("write collector config: %w", err)
	}

	opampClient, err := opamp.NewFromEnv(version, col, collector.GPUConfig)
	if err != nil {
		return fmt.Errorf("create opamp client: %w", err)
	}
	if opampClient != nil {
		if err := opampClient.Start(context.Background()); err != nil {
			return fmt.Errorf("start opamp client: %w", err)
		}
		defer func() {
			stopCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := opampClient.Stop(stopCtx); err != nil {
				slog.Error("opamp stop failed", "err", err)
			}
		}()
	}

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGTERM, syscall.SIGINT)
	<-sigs

	return nil
}

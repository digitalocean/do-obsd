package main

import (
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/digitalocean/do-obsd/internal/collector"
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
	slog.Info("starting", "version", version)

	col := collector.New()

	if err := col.Install(); err != nil {
		return fmt.Errorf("install collector: %w", err)
	}

	if err := col.WriteConfig(collector.GPUConfig); err != nil {
		return fmt.Errorf("write collector config: %w", err)
	}

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGTERM, syscall.SIGINT)
	<-sigs

	return nil
}

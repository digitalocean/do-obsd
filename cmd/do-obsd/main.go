package main

import (
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/digitalocean/do-obsd/internal/collector"
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

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGTERM, syscall.SIGINT)
	<-sigs

	slog.Info("stopping")

	if err := col.Stop(); err != nil {
		slog.Warn("stop collector failed", "err", err)
	}

	return nil
}

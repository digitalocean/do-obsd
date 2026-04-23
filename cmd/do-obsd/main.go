package main

import (
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/digitalocean/do-obsd/internal/supervisor"
	"github.com/digitalocean/do-obsd/internal/vpcendpoint"
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

	ip, err := vpcendpoint.New().Discover()
	if err != nil {
		return fmt.Errorf("discover vpc endpoint: %w", err)
	}
	slog.Info("discovered vpc endpoint", "ip", ip)

	config, err := supervisor.BuildConfig(ip)
	if err != nil {
		return fmt.Errorf("build collector config: %w", err)
	}

	col := supervisor.New()
	if err := col.WriteConfig(config); err != nil {
		return fmt.Errorf("write collector config: %w", err)
	}

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGTERM, syscall.SIGINT)
	<-sigs

	return nil
}

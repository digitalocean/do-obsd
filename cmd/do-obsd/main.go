package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/digitalocean/do-obsd/internal/supervisor"
)

var version = "dev"

func main() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, nil)))
	slog.Info("starting", "version", version)

	dropletType := flag.String("droplet-type", "", "type of droplet this agent is running on (e.g. k8s-node, dbaas)")
	flag.Parse()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	if err := supervisor.New(*dropletType).Run(ctx); err != nil {
		slog.Error("fatal", "err", err)
		os.Exit(1)
	}
}

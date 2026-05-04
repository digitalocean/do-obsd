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

func main() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, nil)))

	versionFlag := flag.Bool("version", false, "print version information and exit")
	dropletType := flag.String("droplet-type", "", "type of droplet this agent is running on (e.g. k8s-node, dbaas)")
	flag.Parse()

	if *versionFlag {
		Print(os.Stdout)
		return
	}

	bi := Info()
	slog.Info("starting",
		"version", bi.Version,
		"revision", bi.Revision,
		"buildDate", bi.BuildDate,
		"goVersion", bi.GoVersion,
	)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	if err := supervisor.New(*dropletType).Run(ctx); err != nil {
		slog.Error("fatal", "err", err)
		os.Exit(1)
	}
}

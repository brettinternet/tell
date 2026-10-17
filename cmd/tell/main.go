package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/brettinternet/tell/internal/cli"
	"github.com/brettinternet/tell/internal/config"
)

var (
	buildVersion = "dev"
	buildCommit  = "unknown"
	buildTime    = "unknown"
)

func main() {
	os.Exit(run())
}

func run() int {
	// Let bounded output handling report EPIPE as an honest failure, rather
	// than the runtime terminating stdout/stderr writers with SIGPIPE.
	signal.Ignore(syscall.SIGPIPE)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM, syscall.SIGHUP)
	defer stop()
	return cli.Run(ctx, os.Args, config.BuildOpts{
		Version: buildVersion, Commit: buildCommit, BuildTime: buildTime,
	}, os.Stdout, os.Stderr)
}

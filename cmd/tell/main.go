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
	signals := make(chan os.Signal, 2)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM, syscall.SIGHUP)
	defer signal.Stop(signals)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	secondSignal := make(chan os.Signal, 1)
	go func() {
		<-signals
		cancel()
		secondSignal <- <-signals
	}()
	ctx = cli.WithSecondSignal(ctx, secondSignal)
	return cli.Run(ctx, os.Args, config.BuildOpts{
		Version: buildVersion, Commit: buildCommit, BuildTime: buildTime,
	}, os.Stdout, os.Stderr)
}

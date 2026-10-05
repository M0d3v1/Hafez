// Command hafez is a Redis-compatible in-memory server.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/M0d3v1/hafez/internal/command"
	"github.com/M0d3v1/hafez/internal/server"
	"github.com/M0d3v1/hafez/internal/store"
)

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	fs := flag.NewFlagSet("hafez", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	port := fs.Int("port", 6379, "TCP port to listen on")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() != 0 {
		fmt.Fprintf(os.Stderr, "unexpected arguments\n")
		return 2
	}
	if *port < 0 || *port > 65535 {
		fmt.Fprintf(os.Stderr, "invalid port %d\n", *port)
		return 2
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	st := store.New()
	disp := command.New()
	if err := register(disp, st); err != nil {
		logger.Error("register commands", "err", err)
		return 1
	}

	srv := &server.Server{
		Addr:    fmt.Sprintf(":%d", *port),
		Handler: disp.Dispatch,
		Logger:  logger,
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	expireDone := make(chan struct{})
	go func() {
		defer close(expireDone)
		st.RunActiveExpire(ctx)
	}()

	err := srv.ListenAndServe(ctx)
	stop()
	select {
	case <-expireDone:
	case <-time.After(2 * time.Second):
		logger.Error("expire loop did not stop")
	}
	if err != nil && !errors.Is(err, context.Canceled) {
		logger.Error("server exited", "err", err)
		return 1
	}
	logger.Info("shutdown complete")
	return 0
}

func register(d *command.Dispatcher, st store.Store) error {
	if err := command.RegisterConn(d); err != nil {
		return err
	}
	if err := command.RegisterStrings(d, st); err != nil {
		return err
	}
	if err := command.RegisterKeys(d, st); err != nil {
		return err
	}
	return command.RegisterExpire(d, st)
}

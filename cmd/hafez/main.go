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
	"github.com/M0d3v1/hafez/internal/persistence"
	"github.com/M0d3v1/hafez/internal/resp"
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
	aofPath := fs.String("aof", "", "append-only file path (empty disables persistence)")
	fsyncName := fs.String("appendfsync", "everysec", "AOF fsync policy: always, everysec, or no")
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
	policy, err := persistence.ParsePolicy(*fsyncName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err)
		return 2
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	st := store.New()
	disp := command.New()
	eng := &command.Engine{Disp: disp}
	if err := register(disp, st, eng); err != nil {
		logger.Error("register commands", "err", err)
		return 1
	}
	if *aofPath != "" {
		aof, err := persistence.Open(*aofPath, policy, st, logger)
		if err != nil {
			logger.Error("open aof", "err", err)
			return 1
		}
		defer func() {
			if err := aof.Close(); err != nil {
				logger.Error("aof close", "err", err)
			}
		}()
		if err := aof.Load(func(args []string) error {
			reply := disp.Dispatch(context.Background(), args)
			if reply.Type == resp.TypeError {
				return errors.New(reply.Str)
			}
			return nil
		}); err != nil {
			logger.Error("load aof", "err", err)
			return 1
		}
		// Expiry deletes are logged after replay. During replay, deadlines are
		// relative to now, and the file already contains the commands that
		// created them.
		st.OnExpire(func(key string) { aof.LogExpire(key) })
		eng.AOF = aof
		logger.Info("aof enabled", "path", *aofPath, "appendfsync", *fsyncName)
	}

	srv := &server.Server{
		Addr:    fmt.Sprintf(":%d", *port),
		Handler: eng.Handle,
		Logger:  logger,
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	expireDone := make(chan struct{})
	go func() {
		defer close(expireDone)
		st.RunActiveExpire(ctx)
	}()

	err = srv.ListenAndServe(ctx)
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

func register(d *command.Dispatcher, st store.Store, eng *command.Engine) error {
	if err := command.RegisterConn(d); err != nil {
		return err
	}
	if err := command.RegisterStrings(d, st); err != nil {
		return err
	}
	if err := command.RegisterKeys(d, st); err != nil {
		return err
	}
	if err := command.RegisterExpire(d, st); err != nil {
		return err
	}
	if err := command.RegisterLists(d, st); err != nil {
		return err
	}
	if err := command.RegisterHashes(d, st); err != nil {
		return err
	}
	return eng.Register(d)
}

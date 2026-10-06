package integration

import (
	"context"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/M0d3v1/hafez/internal/command"
	"github.com/M0d3v1/hafez/internal/server"
	"github.com/M0d3v1/hafez/internal/store"
)

func TestRedisClient(t *testing.T) {
	rdb := startClient(t)
	cmdCtx, cmdCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cmdCancel()

	pong, err := rdb.Ping(cmdCtx).Result()
	if err != nil {
		t.Fatal(err)
	}
	if pong != "PONG" {
		t.Fatalf("PING = %q", pong)
	}

	echo, err := rdb.Echo(cmdCtx, "hello").Result()
	if err != nil {
		t.Fatal(err)
	}
	if echo != "hello" {
		t.Fatalf("ECHO = %q", echo)
	}

	n, err := rdb.Do(cmdCtx, "COMMAND", "COUNT").Int64()
	if err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatalf("COMMAND COUNT = %d", n)
	}

	err = rdb.Do(cmdCtx, "HGET", "foo", "bar").Err()
	if err == nil || !strings.Contains(err.Error(), "unknown command") {
		t.Fatalf("HGET error = %v", err)
	}
}

func startClient(t *testing.T) *redis.Client {
	t.Helper()
	st := store.New()
	d := command.New()
	if err := command.RegisterConn(d); err != nil {
		t.Fatal(err)
	}
	if err := command.RegisterStrings(d, st); err != nil {
		t.Fatal(err)
	}
	if err := command.RegisterKeys(d, st); err != nil {
		t.Fatal(err)
	}
	if err := command.RegisterExpire(d, st); err != nil {
		t.Fatal(err)
	}
	srv := &server.Server{
		Addr:    "127.0.0.1:0",
		Handler: d.Dispatch,
		Logger:  slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	if err := srv.Listen(); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() { errCh <- srv.Serve(ctx) }()
	t.Cleanup(func() {
		cancel()
		select {
		case err := <-errCh:
			if err != nil && err != context.Canceled {
				t.Errorf("serve: %v", err)
			}
		case <-time.After(5 * time.Second):
			t.Error("server did not stop")
		}
	})

	rdb := redis.NewClient(&redis.Options{
		Addr:            srv.Addr,
		Protocol:        2,
		DisableIdentity: true,
	})
	t.Cleanup(func() { _ = rdb.Close() })
	return rdb
}

package integration

import (
	"context"
	"io"
	"log/slog"
	"path/filepath"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/M0d3v1/hafez/internal/command"
	"github.com/M0d3v1/hafez/internal/persistence"
	"github.com/M0d3v1/hafez/internal/resp"
	"github.com/M0d3v1/hafez/internal/server"
	"github.com/M0d3v1/hafez/internal/store"
)

func TestAOFSurvivesRestart(t *testing.T) {
	path := filepath.Join(t.TempDir(), "appendonly.aof")
	addr, wait, stop := startAOF(t, path)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	rdb := redis.NewClient(&redis.Options{Addr: addr, Protocol: 2, DisableIdentity: true})
	if err := rdb.Set(ctx, "s", "hello", 0).Err(); err != nil {
		t.Fatal(err)
	}
	if err := rdb.RPush(ctx, "l", "x", "y").Err(); err != nil {
		t.Fatal(err)
	}
	if err := rdb.HSet(ctx, "h", "f", "v").Err(); err != nil {
		t.Fatal(err)
	}
	if err := rdb.Expire(ctx, "s", 30*time.Second).Err(); err != nil {
		t.Fatal(err)
	}
	if err := rdb.Incr(ctx, "n").Err(); err != nil {
		t.Fatal(err)
	}
	if err := rdb.Incr(ctx, "n").Err(); err != nil {
		t.Fatal(err)
	}
	if err := rdb.Do(ctx, "BGREWRITEAOF").Err(); err != nil {
		t.Fatal(err)
	}
	if err := wait(); err != nil {
		t.Fatal(err)
	}
	if err := rdb.Incr(ctx, "n").Err(); err != nil {
		t.Fatal(err)
	}
	if err := rdb.Close(); err != nil {
		t.Fatal(err)
	}
	stop()

	addr, _, stop = startAOF(t, path)
	defer stop()
	rdb = redis.NewClient(&redis.Options{Addr: addr, Protocol: 2, DisableIdentity: true})
	defer rdb.Close()
	ctx2, cancel2 := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel2()

	if got, err := rdb.Get(ctx2, "s").Result(); err != nil || got != "hello" {
		t.Fatalf("s = %q %v", got, err)
	}
	ttl, err := rdb.TTL(ctx2, "s").Result()
	if err != nil || ttl <= 0 || ttl > 30*time.Second {
		t.Fatalf("ttl = %s %v", ttl, err)
	}
	list, err := rdb.LRange(ctx2, "l", 0, -1).Result()
	if err != nil || len(list) != 2 || list[0] != "x" || list[1] != "y" {
		t.Fatalf("list = %v %v", list, err)
	}
	field, err := rdb.HGet(ctx2, "h", "f").Result()
	if err != nil || field != "v" {
		t.Fatalf("hash = %q %v", field, err)
	}
	n, err := rdb.Get(ctx2, "n").Int64()
	if err != nil || n != 3 {
		t.Fatalf("n = %d %v", n, err)
	}
}

func startAOF(t *testing.T, path string) (addr string, wait func() error, stop func()) {
	t.Helper()
	st := store.New()
	d := command.New()
	eng := &command.Engine{Disp: d}
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
	if err := command.RegisterLists(d, st); err != nil {
		t.Fatal(err)
	}
	if err := command.RegisterHashes(d, st); err != nil {
		t.Fatal(err)
	}
	if err := eng.Register(d); err != nil {
		t.Fatal(err)
	}
	aof, err := persistence.Open(path, persistence.FsyncAlways, st, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := aof.Load(func(args []string) error {
		reply := d.Dispatch(context.Background(), args)
		if reply.Type == resp.TypeError {
			return errString(reply.Str)
		}
		return nil
	}); err != nil {
		aof.Close()
		t.Fatal(err)
	}
	st.OnExpire(func(key string) { aof.LogExpire(key) })
	eng.AOF = aof

	srv := &server.Server{
		Addr:    "127.0.0.1:0",
		Handler: eng.Handle,
		Logger:  slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	if err := srv.Listen(); err != nil {
		aof.Close()
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- srv.Serve(ctx) }()
	return srv.Addr, aof.Wait, func() {
		cancel()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Error("server did not stop")
		}
		aof.Close()
	}
}

type errString string

func (e errString) Error() string { return string(e) }

package command

import (
	"context"
	"errors"
	"path/filepath"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/M0d3v1/hafez/internal/persistence"
	"github.com/M0d3v1/hafez/internal/resp"
	"github.com/M0d3v1/hafez/internal/store"
)

func TestAOFSkipsWritesThatDoNotChangeState(t *testing.T) {
	st := store.New()
	eng, aof, path := openEngine(t, st)
	ctx := context.Background()
	eng.Handle(ctx, []string{"SET", "k", "1"})
	eng.Handle(ctx, []string{"SET", "k", "2", "NX"})
	eng.Handle(ctx, []string{"SET", "word", "hello"})
	eng.Handle(ctx, []string{"INCR", "word"})
	eng.Handle(ctx, []string{"INCR", "k"})
	eng.Handle(ctx, []string{"LPOP", "missing"})
	eng.Handle(ctx, []string{"HDEL", "missing", "f"})
	eng.Handle(ctx, []string{"DEL", "missing"})
	if err := aof.Close(); err != nil {
		t.Fatal(err)
	}

	got := loggedCommands(t, path)
	want := [][]string{
		{"SET", "k", "1"},
		{"SET", "word", "hello"},
		{"INCR", "k"},
	}
	if len(got) != len(want) {
		t.Fatalf("log = %v", got)
	}
	for i := range want {
		if len(got[i]) != len(want[i]) {
			t.Fatalf("log = %v", got)
		}
		for j := range want[i] {
			if got[i][j] != want[i][j] {
				t.Fatalf("log = %v", got)
			}
		}
	}
}

func TestAOFReplayAndExpire(t *testing.T) {
	start := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	now := start
	st := store.New(store.WithClock(func() time.Time { return now }))
	eng, aof, path := openEngine(t, st)
	ctx := context.Background()
	eng.Handle(ctx, []string{"SET", "s", "hello"})
	eng.Handle(ctx, []string{"EXPIRE", "s", "10"})
	eng.Handle(ctx, []string{"RPUSH", "l", "x", "y"})
	eng.Handle(ctx, []string{"HSET", "h", "f", "v"})
	eng.Handle(ctx, []string{"SET", "temp", "1"})
	eng.Handle(ctx, []string{"PEXPIRE", "temp", "1000"})
	now = start.Add(time.Second)
	eng.Handle(ctx, []string{"GET", "temp"})
	eng.Handle(ctx, []string{"SET", "n", "5"})
	eng.Handle(ctx, []string{"PEXPIRE", "n", "1000"})
	now = start.Add(2 * time.Second)
	if reply := eng.Handle(ctx, []string{"INCR", "n"}); reply.Int != 1 {
		t.Fatalf("incr after expiry = %+v", reply)
	}
	if err := aof.Close(); err != nil {
		t.Fatal(err)
	}

	again := store.New(store.WithClock(func() time.Time { return now }))
	replay(t, path, again)
	if v, ok, _ := again.Get("s"); !ok || v != "hello" {
		t.Fatalf("s = %q ok=%v", v, ok)
	}
	rem, exists, expires := again.TTL("s")
	if !exists || !expires || rem != 10*time.Second {
		t.Fatalf("ttl = %s exists=%v expires=%v", rem, exists, expires)
	}
	items, err := again.LRange("l", 0, -1)
	if err != nil || len(items) != 2 || items[1] != "y" {
		t.Fatalf("list = %v %v", items, err)
	}
	fields, err := again.HGetAll("h")
	if err != nil || len(fields) != 1 || fields[0].Value != "v" {
		t.Fatalf("hash = %v %v", fields, err)
	}
	if _, ok, _ := again.Get("temp"); ok {
		t.Fatal("expired key returned after replay")
	}
	if v, ok, _ := again.Get("n"); !ok || v != "1" {
		t.Fatalf("n = %q ok=%v", v, ok)
	}
}

func TestBGRewriteAOF(t *testing.T) {
	st := store.New()
	eng, aof, path := openEngine(t, st)
	ctx := context.Background()
	eng.Handle(ctx, []string{"SET", "a", "1"})
	eng.Handle(ctx, []string{"INCR", "a"})
	eng.Handle(ctx, []string{"INCR", "a"})
	reply := eng.Handle(ctx, []string{"BGREWRITEAOF"})
	if reply.Type != resp.TypeSimpleString || reply.Str != "Background append only file rewriting started" {
		t.Fatalf("reply = %+v", reply)
	}
	if err := aof.Wait(); err != nil {
		t.Fatal(err)
	}
	eng.Handle(ctx, []string{"INCR", "a"})
	if err := aof.Close(); err != nil {
		t.Fatal(err)
	}

	again := store.New()
	replay(t, path, again)
	if v, ok, _ := again.Get("a"); !ok || v != "4" {
		t.Fatalf("a = %q ok=%v", v, ok)
	}
}

func TestBGRewriteDisabled(t *testing.T) {
	d := New()
	eng := &Engine{Disp: d}
	if err := eng.Register(d); err != nil {
		t.Fatal(err)
	}
	reply := eng.Handle(context.Background(), []string{"BGREWRITEAOF"})
	if reply.Type != resp.TypeError {
		t.Fatalf("reply = %+v", reply)
	}
}

func TestRewriteDoesNotDoubleApply(t *testing.T) {
	st := store.New()
	eng, aof, path := openEngine(t, st)
	ctx := context.Background()
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			key := strconv.Itoa(i)
			for n := 0; n < 40; n++ {
				eng.Handle(ctx, []string{"INCR", key})
			}
		}(i)
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		for n := 0; n < 5; n++ {
			if err := aof.Rewrite(); err != nil {
				t.Errorf("rewrite: %v", err)
				return
			}
		}
	}()
	wg.Wait()
	if err := aof.Close(); err != nil {
		t.Fatal(err)
	}

	again := store.New()
	replay(t, path, again)
	for i := 0; i < 8; i++ {
		key := strconv.Itoa(i)
		want, _, err := st.Get(key)
		if err != nil {
			t.Fatal(err)
		}
		got, ok, err := again.Get(key)
		if err != nil || !ok || got != want {
			t.Fatalf("key %s = %q ok=%v err=%v, want %q", key, got, ok, err, want)
		}
	}
}

func openEngine(t *testing.T, st *store.Memory) (*Engine, *persistence.AOF, string) {
	t.Helper()
	d := New()
	if err := registerAll(d, st); err != nil {
		t.Fatal(err)
	}
	eng := &Engine{Disp: d}
	if err := eng.Register(d); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "appendonly.aof")
	aof, err := persistence.Open(path, persistence.FsyncAlways, st, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { aof.Close() })
	if err := aof.Load(func([]string) error { return nil }); err != nil {
		t.Fatal(err)
	}
	st.OnExpire(func(key string) { aof.LogExpire(key) })
	eng.AOF = aof
	return eng, aof, path
}

func loggedCommands(t *testing.T, path string) [][]string {
	t.Helper()
	aof, err := persistence.Open(path, persistence.FsyncNo, store.New(), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer aof.Close()
	var got [][]string
	if err := aof.Load(func(args []string) error {
		got = append(got, append([]string(nil), args...))
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	return got
}

func replay(t *testing.T, path string, st *store.Memory) {
	t.Helper()
	d := New()
	if err := registerAll(d, st); err != nil {
		t.Fatal(err)
	}
	aof, err := persistence.Open(path, persistence.FsyncNo, st, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer aof.Close()
	err = aof.Load(func(args []string) error {
		reply := d.Dispatch(context.Background(), args)
		if reply.Type == resp.TypeError {
			return errors.New(reply.Str)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func registerAll(d *Dispatcher, st store.Store) error {
	if err := RegisterConn(d); err != nil {
		return err
	}
	if err := RegisterStrings(d, st); err != nil {
		return err
	}
	if err := RegisterKeys(d, st); err != nil {
		return err
	}
	if err := RegisterExpire(d, st); err != nil {
		return err
	}
	if err := RegisterLists(d, st); err != nil {
		return err
	}
	return RegisterHashes(d, st)
}

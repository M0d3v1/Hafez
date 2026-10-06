package command

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/M0d3v1/hafez/internal/resp"
	"github.com/M0d3v1/hafez/internal/store"
)

func TestStringCommands(t *testing.T) {
	d, _ := newKeyspace(t)
	ctx := context.Background()

	must(t, d, ctx, []string{"SET", "a", "hello"}, resp.SimpleString("OK"))
	must(t, d, ctx, []string{"GET", "a"}, resp.BulkString("hello"))
	must(t, d, ctx, []string{"GET", "missing"}, resp.NullBulk())
	must(t, d, ctx, []string{"SET", "a", "new", "NX"}, resp.NullBulk())
	must(t, d, ctx, []string{"GET", "a"}, resp.BulkString("hello"))
	must(t, d, ctx, []string{"SET", "missing", "x", "XX"}, resp.NullBulk())
	must(t, d, ctx, []string{"SET", "a", "world", "XX"}, resp.SimpleString("OK"))
	must(t, d, ctx, []string{"SET", "a", "v", "EX", "0"}, expireErr())
	must(t, d, ctx, []string{"SET", "a", "v", "PX", "-1"}, expireErr())
	must(t, d, ctx, []string{"SET", "a", "v", "EX", "nope"}, notIntegerErr())
	must(t, d, ctx, []string{"SET", "a", "v", "EX"}, syntaxErr())
	must(t, d, ctx, []string{"SET", "a", "v", "NX", "XX"}, syntaxErr())
	must(t, d, ctx, []string{"SET", "a", "v", "FOO"}, syntaxErr())
	must(t, d, ctx, []string{"SET"}, resp.Error("ERR wrong number of arguments for 'set' command"))

	must(t, d, ctx, []string{"MSET", "x", "1", "y", "2"}, resp.SimpleString("OK"))
	must(t, d, ctx, []string{"MSET", "x", "1", "y"}, resp.Error("ERR wrong number of arguments for 'mset' command"))
	must(t, d, ctx, []string{"MGET", "x", "missing", "y"}, resp.Array(
		resp.BulkString("1"), resp.NullBulk(), resp.BulkString("2"),
	))

	must(t, d, ctx, []string{"INCR", "n"}, resp.Integer(1))
	must(t, d, ctx, []string{"INCRBY", "n", "4"}, resp.Integer(5))
	must(t, d, ctx, []string{"DECR", "n"}, resp.Integer(4))
	must(t, d, ctx, []string{"SET", "word", "hi"}, resp.SimpleString("OK"))
	must(t, d, ctx, []string{"INCR", "word"}, notIntegerErr())
	must(t, d, ctx, []string{"INCRBY", "n", "1.5"}, notIntegerErr())
	must(t, d, ctx, []string{"GET", "word"}, resp.BulkString("hi"))
}

func TestSetExpiryThroughClock(t *testing.T) {
	start := time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)
	now := start
	d, _ := newKeyspace(t, store.WithClock(func() time.Time { return now }))
	ctx := context.Background()
	must(t, d, ctx, []string{"SET", "k", "v", "EX", "2"}, resp.SimpleString("OK"))
	must(t, d, ctx, []string{"GET", "k"}, resp.BulkString("v"))
	now = start.Add(2 * time.Second)
	must(t, d, ctx, []string{"GET", "k"}, resp.NullBulk())

	now = start
	must(t, d, ctx, []string{"SET", "k", "v", "px", "10"}, resp.SimpleString("OK"))
	now = start.Add(10 * time.Millisecond)
	must(t, d, ctx, []string{"GET", "k"}, resp.NullBulk())
}

func TestReplyErr(t *testing.T) {
	got := replyErr(store.ErrWrongType)
	want := resp.Error("WRONGTYPE Operation against a key holding the wrong kind of value")
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("%#v", got)
	}
	if !reflect.DeepEqual(replyErr(store.ErrNotInteger), notIntegerErr()) {
		t.Fatal("integer mapping")
	}
}

func newKeyspace(t *testing.T, opts ...store.Option) (*Dispatcher, *store.Memory) {
	t.Helper()
	m := store.New(opts...)
	d := New()
	if err := RegisterStrings(d, m); err != nil {
		t.Fatal(err)
	}
	if err := RegisterKeys(d, m); err != nil {
		t.Fatal(err)
	}
	if err := RegisterExpire(d, m); err != nil {
		t.Fatal(err)
	}
	if err := RegisterLists(d, m); err != nil {
		t.Fatal(err)
	}
	if err := RegisterHashes(d, m); err != nil {
		t.Fatal(err)
	}
	return d, m
}

func must(t *testing.T, d *Dispatcher, ctx context.Context, args []string, want resp.Value) {
	t.Helper()
	got := d.Dispatch(ctx, args)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Dispatch(%q) = %#v, want %#v", args, got, want)
	}
}

package command

import (
	"context"
	"testing"
	"time"

	"github.com/M0d3v1/hafez/internal/resp"
	"github.com/M0d3v1/hafez/internal/store"
)

func TestExpireCommands(t *testing.T) {
	start := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	now := start
	d, _ := newKeyspace(t, store.WithClock(func() time.Time { return now }))
	ctx := context.Background()

	must(t, d, ctx, []string{"EXPIRE", "missing", "10"}, resp.Integer(0))
	must(t, d, ctx, []string{"TTL", "missing"}, resp.Integer(-2))
	must(t, d, ctx, []string{"PTTL", "missing"}, resp.Integer(-2))
	must(t, d, ctx, []string{"PERSIST", "missing"}, resp.Integer(0))

	must(t, d, ctx, []string{"SET", "k", "v"}, resp.SimpleString("OK"))
	must(t, d, ctx, []string{"TTL", "k"}, resp.Integer(-1))
	must(t, d, ctx, []string{"PERSIST", "k"}, resp.Integer(0))
	must(t, d, ctx, []string{"EXPIRE", "k", "10"}, resp.Integer(1))
	must(t, d, ctx, []string{"TTL", "k"}, resp.Integer(10))
	must(t, d, ctx, []string{"PTTL", "k"}, resp.Integer(10000))
	must(t, d, ctx, []string{"PERSIST", "k"}, resp.Integer(1))
	must(t, d, ctx, []string{"TTL", "k"}, resp.Integer(-1))
	must(t, d, ctx, []string{"GET", "k"}, resp.BulkString("v"))

	must(t, d, ctx, []string{"PEXPIRE", "k", "1500"}, resp.Integer(1))
	must(t, d, ctx, []string{"PTTL", "k"}, resp.Integer(1500))
	must(t, d, ctx, []string{"TTL", "k"}, resp.Integer(1))

	must(t, d, ctx, []string{"EXPIRE", "k", "0"}, resp.Integer(1))
	must(t, d, ctx, []string{"GET", "k"}, resp.NullBulk())
	must(t, d, ctx, []string{"SET", "k", "v"}, resp.SimpleString("OK"))
	must(t, d, ctx, []string{"EXPIRE", "k", "-1"}, resp.Integer(1))
	must(t, d, ctx, []string{"TTL", "k"}, resp.Integer(-2))

	must(t, d, ctx, []string{"SET", "k", "v"}, resp.SimpleString("OK"))
	must(t, d, ctx, []string{"EXPIRE", "k", "nope"}, notIntegerErr())
	must(t, d, ctx, []string{"EXPIRE", "k"}, resp.Error("ERR wrong number of arguments for 'expire' command"))

	must(t, d, ctx, []string{"EXPIRE", "k", "2"}, resp.Integer(1))
	now = start.Add(2 * time.Second)
	must(t, d, ctx, []string{"TTL", "k"}, resp.Integer(-2))
	must(t, d, ctx, []string{"GET", "k"}, resp.NullBulk())
}

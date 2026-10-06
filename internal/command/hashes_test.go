package command

import (
	"context"
	"testing"

	"github.com/M0d3v1/hafez/internal/resp"
)

func TestHashCommands(t *testing.T) {
	d, _ := newKeyspace(t)
	ctx := context.Background()
	wrong := resp.Error("WRONGTYPE Operation against a key holding the wrong kind of value")

	must(t, d, ctx, []string{"HSET", "h", "a", "1", "b", "2"}, resp.Integer(2))
	must(t, d, ctx, []string{"HSET", "h", "a", "3"}, resp.Integer(0))
	must(t, d, ctx, []string{"HGET", "h", "a"}, resp.BulkString("3"))
	must(t, d, ctx, []string{"HGET", "h", "missing"}, resp.NullBulk())
	must(t, d, ctx, []string{"HGET", "missing", "a"}, resp.NullBulk())
	must(t, d, ctx, []string{"HEXISTS", "h", "b"}, resp.Integer(1))
	must(t, d, ctx, []string{"HEXISTS", "h", "nope"}, resp.Integer(0))
	must(t, d, ctx, []string{"HLEN", "h"}, resp.Integer(2))
	must(t, d, ctx, []string{"HLEN", "missing"}, resp.Integer(0))
	must(t, d, ctx, []string{"HGETALL", "h"}, resp.Array(
		resp.BulkString("a"), resp.BulkString("3"),
		resp.BulkString("b"), resp.BulkString("2"),
	))
	must(t, d, ctx, []string{"HGETALL", "missing"}, resp.Array())
	must(t, d, ctx, []string{"HDEL", "h", "b", "missing"}, resp.Integer(1))
	must(t, d, ctx, []string{"HDEL", "h", "a"}, resp.Integer(1))
	must(t, d, ctx, []string{"EXISTS", "h"}, resp.Integer(0))

	must(t, d, ctx, []string{"HSET", "h", "a", "1", "b"}, resp.Error("ERR wrong number of arguments for 'hset' command"))
	must(t, d, ctx, []string{"SET", "s", "v"}, resp.SimpleString("OK"))
	must(t, d, ctx, []string{"HSET", "s", "a", "1"}, wrong)
	must(t, d, ctx, []string{"HSET", "h", "a", "1"}, resp.Integer(1))
	must(t, d, ctx, []string{"TYPE", "h"}, resp.SimpleString("hash"))
	must(t, d, ctx, []string{"LPUSH", "h", "x"}, wrong)
}

package command

import (
	"context"
	"testing"

	"github.com/M0d3v1/hafez/internal/resp"
)

func TestListCommands(t *testing.T) {
	d, _ := newKeyspace(t)
	ctx := context.Background()
	wrong := resp.Error("WRONGTYPE Operation against a key holding the wrong kind of value")

	must(t, d, ctx, []string{"LPUSH", "l", "a", "b", "c"}, resp.Integer(3))
	must(t, d, ctx, []string{"LRANGE", "l", "0", "-1"}, resp.Array(
		resp.BulkString("c"), resp.BulkString("b"), resp.BulkString("a"),
	))
	must(t, d, ctx, []string{"RPUSH", "l", "d"}, resp.Integer(4))
	must(t, d, ctx, []string{"LLEN", "l"}, resp.Integer(4))
	must(t, d, ctx, []string{"LLEN", "missing"}, resp.Integer(0))
	must(t, d, ctx, []string{"LPOP", "l"}, resp.BulkString("c"))
	must(t, d, ctx, []string{"RPOP", "l"}, resp.BulkString("d"))
	must(t, d, ctx, []string{"LPOP", "missing"}, resp.NullBulk())
	must(t, d, ctx, []string{"LRANGE", "missing", "0", "-1"}, resp.Array())
	must(t, d, ctx, []string{"LRANGE", "l", "nope", "1"}, notIntegerErr())

	must(t, d, ctx, []string{"SET", "s", "v"}, resp.SimpleString("OK"))
	must(t, d, ctx, []string{"LPUSH", "s", "a"}, wrong)
	must(t, d, ctx, []string{"TYPE", "l"}, resp.SimpleString("list"))
	must(t, d, ctx, []string{"GET", "l"}, wrong)

	must(t, d, ctx, []string{"LPOP", "l"}, resp.BulkString("b"))
	must(t, d, ctx, []string{"LPOP", "l"}, resp.BulkString("a"))
	must(t, d, ctx, []string{"LPOP", "l"}, resp.NullBulk())
	must(t, d, ctx, []string{"EXISTS", "l"}, resp.Integer(0))
}

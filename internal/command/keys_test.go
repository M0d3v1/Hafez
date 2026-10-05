package command

import (
	"context"
	"reflect"
	"sort"
	"testing"

	"github.com/M0d3v1/hafez/internal/resp"
)

func TestKeyCommands(t *testing.T) {
	d, _ := newKeyspace(t)
	ctx := context.Background()

	must(t, d, ctx, []string{"DBSIZE"}, resp.Integer(0))
	must(t, d, ctx, []string{"SET", "user:1", "a"}, resp.SimpleString("OK"))
	must(t, d, ctx, []string{"SET", "user:2", "b"}, resp.SimpleString("OK"))
	must(t, d, ctx, []string{"SET", "other", "c"}, resp.SimpleString("OK"))
	must(t, d, ctx, []string{"EXISTS", "user:1", "nope", "user:1"}, resp.Integer(2))
	must(t, d, ctx, []string{"TYPE", "user:1"}, resp.SimpleString("string"))
	must(t, d, ctx, []string{"TYPE", "nope"}, resp.SimpleString("none"))
	must(t, d, ctx, []string{"DBSIZE"}, resp.Integer(3))

	got := d.Dispatch(ctx, []string{"KEYS", "user:*"})
	var names []string
	for _, v := range got.Array {
		names = append(names, v.Str)
	}
	sort.Strings(names)
	if !reflect.DeepEqual(names, []string{"user:1", "user:2"}) {
		t.Fatalf("KEYS = %v", names)
	}
	must(t, d, ctx, []string{"KEYS", "none"}, resp.Array())

	must(t, d, ctx, []string{"DEL", "user:1", "missing", "user:1"}, resp.Integer(1))
	must(t, d, ctx, []string{"DBSIZE"}, resp.Integer(2))
	must(t, d, ctx, []string{"FLUSHALL"}, resp.SimpleString("OK"))
	must(t, d, ctx, []string{"DBSIZE"}, resp.Integer(0))
	must(t, d, ctx, []string{"FLUSHALL", "ASYNC"}, resp.Error("ERR wrong number of arguments for 'flushall' command"))
}

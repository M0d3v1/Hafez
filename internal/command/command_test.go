package command

import (
	"context"
	"reflect"
	"testing"

	"github.com/M0d3v1/hafez/internal/resp"
)

func TestDispatchConnCommands(t *testing.T) {
	d := newConnDispatcher(t)
	ctx := context.Background()

	tests := []struct {
		args []string
		want resp.Value
	}{
		{args: []string{"PING"}, want: resp.SimpleString("PONG")},
		{args: []string{"ping"}, want: resp.SimpleString("PONG")},
		{args: []string{"PING", "hello"}, want: resp.BulkString("hello")},
		{args: []string{"ECHO", "hello"}, want: resp.BulkString("hello")},
		{args: []string{"echo", ""}, want: resp.BulkString("")},
		{args: []string{"COMMAND"}, want: resp.Array()},
		{args: []string{"command", "COUNT"}, want: resp.Integer(0)},
		{args: []string{"COMMAND", "docs"}, want: resp.Array()},
		{args: []string{"COMMAND", "INFO", "ping"}, want: resp.Array()},
		{args: []string{"COMMAND", "LIST"}, want: resp.Array()},
		{args: []string{"COMMAND", "GETKEYS", "GET", "a"}, want: resp.Array()},
		{args: []string{"GET", "foo"}, want: resp.Error("ERR unknown command 'GET'")},
		{args: []string{"nope"}, want: resp.Error("ERR unknown command 'nope'")},
		{args: []string{"ECHO"}, want: resp.Error("ERR wrong number of arguments for 'echo' command")},
		{args: []string{"PING", "a", "b"}, want: resp.Error("ERR wrong number of arguments for 'ping' command")},
		{args: []string{"COMMAND", "COUNT", "extra"}, want: resp.Error("ERR wrong number of arguments for 'command|count' command")},
		{args: []string{"COMMAND", "FOO"}, want: resp.Error("ERR unknown subcommand 'FOO'. Try COMMAND HELP.")},
		{args: []string{"bad\rname"}, want: resp.Error("ERR unknown command '?'")},
	}

	for _, tt := range tests {
		got := d.Dispatch(ctx, tt.args)
		if !reflect.DeepEqual(got, tt.want) {
			t.Errorf("Dispatch(%q) = %#v, want %#v", tt.args, got, tt.want)
		}
	}
}

func TestRegisterValidation(t *testing.T) {
	d := New()
	fn := func(context.Context, []string) resp.Value { return resp.SimpleString("OK") }
	if err := d.Register("", 0, 0, fn); err == nil {
		t.Fatal("empty name was accepted")
	}
	if err := d.Register("ok", 0, 0, nil); err == nil {
		t.Fatal("nil handler was accepted")
	}
	if err := d.Register("ok", 2, 1, fn); err == nil {
		t.Fatal("min greater than max was accepted")
	}
	if err := d.Register("ok", -1, 1, fn); err == nil {
		t.Fatal("negative min was accepted")
	}
	if err := d.Register("ok", 0, 1, fn); err != nil {
		t.Fatal(err)
	}
	if err := d.Register("OK", 0, 0, fn); err == nil {
		t.Fatal("duplicate name was accepted")
	}
}

func newConnDispatcher(t *testing.T) *Dispatcher {
	t.Helper()
	d := New()
	if err := RegisterConn(d); err != nil {
		t.Fatal(err)
	}
	return d
}

package command

import (
	"context"
	"strings"

	"github.com/M0d3v1/hafez/internal/resp"
)

// RegisterConn registers the connection commands served in phase 1.
func RegisterConn(d *Dispatcher) error {
	if err := d.Register("ping", 0, 1, ping); err != nil {
		return err
	}
	if err := d.Register("echo", 1, 1, echo); err != nil {
		return err
	}
	return d.Register("command", 0, -1, commandCmd)
}

func ping(_ context.Context, args []string) resp.Value {
	if len(args) == 1 {
		return resp.SimpleString("PONG")
	}
	return resp.BulkString(args[1])
}

func echo(_ context.Context, args []string) resp.Value {
	return resp.BulkString(args[1])
}

// commandCmd is a stub. It advertises an empty command table so redis-cli can
// connect and fall back to RESP2 without caching a partial command list.
// COMMAND, COMMAND COUNT, and the read-only introspection subcommands are
// answered; every other subcommand is an error.
func commandCmd(_ context.Context, args []string) resp.Value {
	if len(args) == 1 {
		return resp.Array()
	}
	switch strings.ToUpper(args[1]) {
	case "COUNT":
		if len(args) != 2 {
			return resp.Error("ERR wrong number of arguments for 'command|count' command")
		}
		return resp.Integer(0)
	case "INFO", "DOCS", "LIST", "GETKEYS":
		return resp.Array()
	default:
		return resp.Error("ERR unknown subcommand '" + safeToken(args[1]) + "'. Try COMMAND HELP.")
	}
}

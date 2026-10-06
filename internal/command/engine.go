package command

import (
	"context"
	"errors"

	"github.com/M0d3v1/hafez/internal/persistence"
	"github.com/M0d3v1/hafez/internal/resp"
)

// Engine runs commands and appends the ones that change the keyspace.
type Engine struct {
	Disp *Dispatcher
	AOF  *persistence.AOF
}

// Handle dispatches one command. Write commands are logged when they mutate
// the store. A nil AOF only dispatches.
func (e *Engine) Handle(ctx context.Context, args []string) resp.Value {
	if e.AOF == nil || len(args) == 0 || !IsWrite(args[0]) {
		return e.Disp.Dispatch(ctx, args)
	}
	var reply resp.Value
	e.AOF.Record(args, func() bool {
		reply = e.Disp.Dispatch(ctx, args)
		return Mutated(args[0], reply)
	})
	return reply
}

// Register adds BGREWRITEAOF.
func (e *Engine) Register(d *Dispatcher) error {
	return d.Register("bgrewriteaof", 0, 0, e.bgRewrite)
}

func (e *Engine) bgRewrite(ctx context.Context, args []string) resp.Value {
	if e.AOF == nil {
		return resp.Error("ERR Append only file is disabled")
	}
	if err := e.AOF.BeginRewrite(); err != nil {
		if errors.Is(err, persistence.ErrRewriteBusy) {
			return resp.Error("ERR Background append only file rewriting already in progress")
		}
		return resp.Error("ERR " + err.Error())
	}
	return resp.SimpleString("Background append only file rewriting started")
}

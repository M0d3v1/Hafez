package command

import (
	"context"
	"strconv"

	"github.com/M0d3v1/hafez/internal/resp"
	"github.com/M0d3v1/hafez/internal/store"
)

// RegisterLists registers LPUSH, RPUSH, LPOP, RPOP, LRANGE, and LLEN.
func RegisterLists(d *Dispatcher, st store.Store) error {
	h := &listCmds{st: st}
	if err := d.Register("lpush", 2, -1, h.lpush); err != nil {
		return err
	}
	if err := d.Register("rpush", 2, -1, h.rpush); err != nil {
		return err
	}
	if err := d.Register("lpop", 1, 1, h.lpop); err != nil {
		return err
	}
	if err := d.Register("rpop", 1, 1, h.rpop); err != nil {
		return err
	}
	if err := d.Register("lrange", 3, 3, h.lrange); err != nil {
		return err
	}
	return d.Register("llen", 1, 1, h.llen)
}

type listCmds struct {
	st store.Store
}

func (h *listCmds) lpush(_ context.Context, args []string) resp.Value {
	n, err := h.st.LPush(args[1], args[2:])
	if err != nil {
		return replyErr(err)
	}
	return resp.Integer(int64(n))
}

func (h *listCmds) rpush(_ context.Context, args []string) resp.Value {
	n, err := h.st.RPush(args[1], args[2:])
	if err != nil {
		return replyErr(err)
	}
	return resp.Integer(int64(n))
}

func (h *listCmds) lpop(_ context.Context, args []string) resp.Value {
	return h.pop(h.st.LPop(args[1]))
}

func (h *listCmds) rpop(_ context.Context, args []string) resp.Value {
	return h.pop(h.st.RPop(args[1]))
}

func (h *listCmds) pop(value string, ok bool, err error) resp.Value {
	if err != nil {
		return replyErr(err)
	}
	if !ok {
		return resp.NullBulk()
	}
	return resp.BulkString(value)
}

func (h *listCmds) lrange(_ context.Context, args []string) resp.Value {
	start, err := strconv.ParseInt(args[2], 10, 64)
	if err != nil {
		return notIntegerErr()
	}
	stop, err := strconv.ParseInt(args[3], 10, 64)
	if err != nil {
		return notIntegerErr()
	}
	vals, err := h.st.LRange(args[1], start, stop)
	if err != nil {
		return replyErr(err)
	}
	out := make([]resp.Value, len(vals))
	for i, v := range vals {
		out[i] = resp.BulkString(v)
	}
	return resp.Array(out...)
}

func (h *listCmds) llen(_ context.Context, args []string) resp.Value {
	n, err := h.st.LLen(args[1])
	if err != nil {
		return replyErr(err)
	}
	return resp.Integer(int64(n))
}

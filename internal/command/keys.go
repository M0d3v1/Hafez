package command

import (
	"context"

	"github.com/M0d3v1/hafez/internal/resp"
	"github.com/M0d3v1/hafez/internal/store"
)

// RegisterKeys registers DEL, EXISTS, KEYS, TYPE, FLUSHALL, and DBSIZE.
func RegisterKeys(d *Dispatcher, st store.Store) error {
	h := &keyCmds{st: st}
	if err := d.Register("del", 1, -1, h.del); err != nil {
		return err
	}
	if err := d.Register("exists", 1, -1, h.exists); err != nil {
		return err
	}
	if err := d.Register("keys", 1, 1, h.keys); err != nil {
		return err
	}
	if err := d.Register("type", 1, 1, h.typ); err != nil {
		return err
	}
	if err := d.Register("flushall", 0, 0, h.flushAll); err != nil {
		return err
	}
	return d.Register("dbsize", 0, 0, h.dbSize)
}

type keyCmds struct {
	st store.Store
}

func (h *keyCmds) del(_ context.Context, args []string) resp.Value {
	return resp.Integer(int64(h.st.Del(args[1:])))
}

func (h *keyCmds) exists(_ context.Context, args []string) resp.Value {
	return resp.Integer(int64(h.st.Exists(args[1:])))
}

func (h *keyCmds) keys(_ context.Context, args []string) resp.Value {
	keys := h.st.Keys(args[1])
	out := make([]resp.Value, len(keys))
	for i, key := range keys {
		out[i] = resp.BulkString(key)
	}
	return resp.Array(out...)
}

func (h *keyCmds) typ(_ context.Context, args []string) resp.Value {
	return resp.SimpleString(h.st.Type(args[1]))
}

func (h *keyCmds) flushAll(_ context.Context, _ []string) resp.Value {
	h.st.FlushAll()
	return resp.SimpleString("OK")
}

func (h *keyCmds) dbSize(_ context.Context, _ []string) resp.Value {
	return resp.Integer(int64(h.st.DBSize()))
}

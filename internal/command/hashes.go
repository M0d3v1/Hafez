package command

import (
	"context"

	"github.com/M0d3v1/hafez/internal/resp"
	"github.com/M0d3v1/hafez/internal/store"
)

// RegisterHashes registers HSET, HGET, HDEL, HGETALL, HEXISTS, and HLEN.
func RegisterHashes(d *Dispatcher, st store.Store) error {
	h := &hashCmds{st: st}
	if err := d.Register("hset", 3, -1, h.hset); err != nil {
		return err
	}
	if err := d.Register("hget", 2, 2, h.hget); err != nil {
		return err
	}
	if err := d.Register("hdel", 2, -1, h.hdel); err != nil {
		return err
	}
	if err := d.Register("hgetall", 1, 1, h.hgetall); err != nil {
		return err
	}
	if err := d.Register("hexists", 2, 2, h.hexists); err != nil {
		return err
	}
	return d.Register("hlen", 1, 1, h.hlen)
}

type hashCmds struct {
	st store.Store
}

func (h *hashCmds) hset(_ context.Context, args []string) resp.Value {
	rest := args[2:]
	if len(rest)%2 != 0 {
		return resp.Error("ERR wrong number of arguments for 'hset' command")
	}
	fields := make([]store.Field, 0, len(rest)/2)
	for i := 0; i < len(rest); i += 2 {
		fields = append(fields, store.Field{Name: rest[i], Value: rest[i+1]})
	}
	n, err := h.st.HSet(args[1], fields)
	if err != nil {
		return replyErr(err)
	}
	return resp.Integer(int64(n))
}

func (h *hashCmds) hget(_ context.Context, args []string) resp.Value {
	v, ok, err := h.st.HGet(args[1], args[2])
	if err != nil {
		return replyErr(err)
	}
	if !ok {
		return resp.NullBulk()
	}
	return resp.BulkString(v)
}

func (h *hashCmds) hdel(_ context.Context, args []string) resp.Value {
	n, err := h.st.HDel(args[1], args[2:])
	if err != nil {
		return replyErr(err)
	}
	return resp.Integer(int64(n))
}

func (h *hashCmds) hgetall(_ context.Context, args []string) resp.Value {
	fields, err := h.st.HGetAll(args[1])
	if err != nil {
		return replyErr(err)
	}
	out := make([]resp.Value, 0, len(fields)*2)
	for _, f := range fields {
		out = append(out, resp.BulkString(f.Name), resp.BulkString(f.Value))
	}
	return resp.Array(out...)
}

func (h *hashCmds) hexists(_ context.Context, args []string) resp.Value {
	ok, err := h.st.HExists(args[1], args[2])
	if err != nil {
		return replyErr(err)
	}
	if ok {
		return resp.Integer(1)
	}
	return resp.Integer(0)
}

func (h *hashCmds) hlen(_ context.Context, args []string) resp.Value {
	n, err := h.st.HLen(args[1])
	if err != nil {
		return replyErr(err)
	}
	return resp.Integer(int64(n))
}

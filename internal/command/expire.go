package command

import (
	"context"
	"strconv"
	"time"

	"github.com/M0d3v1/hafez/internal/resp"
	"github.com/M0d3v1/hafez/internal/store"
)

// RegisterExpire registers EXPIRE, PEXPIRE, TTL, PTTL, and PERSIST.
func RegisterExpire(d *Dispatcher, st store.Store) error {
	h := &expireCmds{st: st}
	if err := d.Register("expire", 2, 2, h.expire); err != nil {
		return err
	}
	if err := d.Register("pexpire", 2, 2, h.pexpire); err != nil {
		return err
	}
	if err := d.Register("ttl", 1, 1, h.ttl); err != nil {
		return err
	}
	if err := d.Register("pttl", 1, 1, h.pttl); err != nil {
		return err
	}
	return d.Register("persist", 1, 1, h.persist)
}

type expireCmds struct {
	st store.Store
}

func (h *expireCmds) expire(_ context.Context, args []string) resp.Value {
	return h.setExpire(args[1], args[2], "EX")
}

func (h *expireCmds) pexpire(_ context.Context, args []string) resp.Value {
	return h.setExpire(args[1], args[2], "PX")
}

func (h *expireCmds) setExpire(key, raw, unit string) resp.Value {
	n, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return notIntegerErr()
	}
	ttl := time.Duration(0)
	if n > 0 {
		d, ok := ttlDuration(unit, n)
		if !ok {
			return notIntegerErr()
		}
		ttl = d
	}
	if h.st.Expire(key, ttl) {
		return resp.Integer(1)
	}
	return resp.Integer(0)
}

func (h *expireCmds) ttl(_ context.Context, args []string) resp.Value {
	return h.report(args[1], time.Second)
}

func (h *expireCmds) pttl(_ context.Context, args []string) resp.Value {
	return h.report(args[1], time.Millisecond)
}

func (h *expireCmds) report(key string, unit time.Duration) resp.Value {
	rem, exists, expires := h.st.TTL(key)
	if !exists {
		return resp.Integer(-2)
	}
	if !expires {
		return resp.Integer(-1)
	}
	n := rem / unit
	if n < 0 {
		n = 0
	}
	return resp.Integer(int64(n))
}

func (h *expireCmds) persist(_ context.Context, args []string) resp.Value {
	if h.st.Persist(args[1]) {
		return resp.Integer(1)
	}
	return resp.Integer(0)
}

package command

import (
	"context"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/M0d3v1/hafez/internal/resp"
	"github.com/M0d3v1/hafez/internal/store"
)

// RegisterStrings registers SET, GET, MSET, MGET, INCR, DECR, and INCRBY.
func RegisterStrings(d *Dispatcher, st store.Store) error {
	h := &stringCmds{st: st}
	if err := d.Register("set", 2, -1, h.set); err != nil {
		return err
	}
	if err := d.Register("get", 1, 1, h.get); err != nil {
		return err
	}
	if err := d.Register("mset", 2, -1, h.mset); err != nil {
		return err
	}
	if err := d.Register("mget", 1, -1, h.mget); err != nil {
		return err
	}
	if err := d.Register("incr", 1, 1, h.incr); err != nil {
		return err
	}
	if err := d.Register("decr", 1, 1, h.decr); err != nil {
		return err
	}
	return d.Register("incrby", 2, 2, h.incrby)
}

type stringCmds struct {
	st store.Store
}

func (h *stringCmds) set(_ context.Context, args []string) resp.Value {
	opt, errVal, ok := parseSetArgs(args[3:])
	if !ok {
		return errVal
	}
	if !h.st.Set(args[1], args[2], opt) {
		return resp.NullBulk()
	}
	return resp.SimpleString("OK")
}

func (h *stringCmds) get(_ context.Context, args []string) resp.Value {
	v, ok, err := h.st.Get(args[1])
	if err != nil {
		return replyErr(err)
	}
	if !ok {
		return resp.NullBulk()
	}
	return resp.BulkString(v)
}

func (h *stringCmds) mset(_ context.Context, args []string) resp.Value {
	rest := args[1:]
	if len(rest)%2 != 0 {
		return resp.Error("ERR wrong number of arguments for 'mset' command")
	}
	pairs := make([]store.Pair, 0, len(rest)/2)
	for i := 0; i < len(rest); i += 2 {
		pairs = append(pairs, store.Pair{Key: rest[i], Value: rest[i+1]})
	}
	h.st.MSet(pairs)
	return resp.SimpleString("OK")
}

func (h *stringCmds) mget(_ context.Context, args []string) resp.Value {
	items, err := h.st.MGet(args[1:])
	if err != nil {
		return replyErr(err)
	}
	out := make([]resp.Value, len(items))
	for i, item := range items {
		if item.Null {
			out[i] = resp.NullBulk()
			continue
		}
		out[i] = resp.BulkString(item.Value)
	}
	return resp.Array(out...)
}

func (h *stringCmds) incr(_ context.Context, args []string) resp.Value {
	return h.incrBy(args[1], 1)
}

func (h *stringCmds) decr(_ context.Context, args []string) resp.Value {
	return h.incrBy(args[1], -1)
}

func (h *stringCmds) incrby(_ context.Context, args []string) resp.Value {
	delta, err := strconv.ParseInt(args[2], 10, 64)
	if err != nil {
		return notIntegerErr()
	}
	return h.incrBy(args[1], delta)
}

func (h *stringCmds) incrBy(key string, delta int64) resp.Value {
	n, err := h.st.IncrBy(key, delta)
	if err != nil {
		return replyErr(err)
	}
	return resp.Integer(n)
}

func parseSetArgs(args []string) (store.SetOptions, resp.Value, bool) {
	var opt store.SetOptions
	for i := 0; i < len(args); i++ {
		switch strings.ToUpper(args[i]) {
		case "NX":
			if opt.NX || opt.XX {
				return store.SetOptions{}, syntaxErr(), false
			}
			opt.NX = true
		case "XX":
			if opt.NX || opt.XX {
				return store.SetOptions{}, syntaxErr(), false
			}
			opt.XX = true
		case "EX", "PX":
			if opt.HasTTL || i+1 >= len(args) {
				return store.SetOptions{}, syntaxErr(), false
			}
			unit := strings.ToUpper(args[i])
			i++
			n, err := strconv.ParseInt(args[i], 10, 64)
			if err != nil {
				return store.SetOptions{}, notIntegerErr(), false
			}
			if n <= 0 {
				return store.SetOptions{}, expireErr(), false
			}
			d, ok := ttlDuration(unit, n)
			if !ok {
				return store.SetOptions{}, expireErr(), false
			}
			opt.TTL = d
			opt.HasTTL = true
		default:
			return store.SetOptions{}, syntaxErr(), false
		}
	}
	return opt, resp.Value{}, true
}

func ttlDuration(unit string, n int64) (time.Duration, bool) {
	unitDur := time.Second
	if unit == "PX" {
		unitDur = time.Millisecond
	}
	if n > int64(math.MaxInt64/int64(unitDur)) {
		return 0, false
	}
	return time.Duration(n) * unitDur, true
}

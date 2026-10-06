// Package persistence writes an append-only file of Redis commands and
// rebuilds the keyspace from that file.
package persistence

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/M0d3v1/hafez/internal/resp"
	"github.com/M0d3v1/hafez/internal/store"
)

// Policy is how often the append-only file is synced to disk.
type Policy int

const (
	// FsyncEverySec syncs about once a second when new commands were written.
	FsyncEverySec Policy = iota
	// FsyncAlways syncs after every logged command.
	FsyncAlways
	// FsyncNo leaves syncing to the operating system until the file is closed.
	FsyncNo
)

// ErrRewriteBusy is returned when a background rewrite is already running.
var ErrRewriteBusy = errors.New("background append only file rewriting already in progress")

// Snapshotter copies the live keyspace for a rewrite.
type Snapshotter interface {
	Snapshot() []store.KeyState
}

// AOF is an append-only command log.
//
// Lock order is op, then the store's shard locks, then mu. Record holds op
// across the command so a snapshot cannot see a write that is not yet in the
// file. A rewrite takes op exclusively, marks the diff buffer, copies the
// keyspace, and only then writes that copy to disk. Commands that arrive
// during the disk write land in the diff and are appended to the new file
// before it replaces the old one.
type AOF struct {
	path   string
	policy Policy
	snap   Snapshotter
	logger *slog.Logger

	op sync.RWMutex // held shared for a write command, exclusive during the snapshot
	mu sync.Mutex
	f  *os.File

	rewriting bool
	running   bool
	diff      bytes.Buffer
	dirty     bool
	notify    chan error
	wg        sync.WaitGroup

	cancel  context.CancelFunc
	stopped chan struct{}
}

// ParsePolicy parses always, everysec, or no.
func ParsePolicy(s string) (Policy, error) {
	switch strings.ToLower(s) {
	case "everysec":
		return FsyncEverySec, nil
	case "always":
		return FsyncAlways, nil
	case "no":
		return FsyncNo, nil
	default:
		return 0, fmt.Errorf("invalid appendfsync %q", s)
	}
}

// Open creates or opens path and starts the everysec sync loop. Call Load
// before serving clients so a partial tail is truncated before new writes.
func Open(path string, policy Policy, snap Snapshotter, logger *slog.Logger) (*AOF, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithCancel(context.Background())
	a := &AOF{
		path:    path,
		policy:  policy,
		snap:    snap,
		logger:  logger,
		f:       f,
		cancel:  cancel,
		stopped: make(chan struct{}),
	}
	go a.fsyncLoop(ctx)
	return a, nil
}

// Load replays complete commands. A truncated final command is discarded and
// the file is cut back to the last good byte. A malformed command that was
// fully written is returned as an error.
func (a *AOF) Load(apply func(args []string) error) error {
	if _, err := a.f.Seek(0, io.SeekStart); err != nil {
		return err
	}
	cr := &countingReader{r: a.f}
	br := bufio.NewReader(cr)
	rd := resp.NewReader(br)

	var good int64
	for {
		start := cr.n - int64(br.Buffered())
		v, err := rd.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			if errors.Is(err, io.ErrUnexpectedEOF) {
				return a.truncate(start)
			}
			return err
		}
		args, err := commandArgs(v)
		if err != nil {
			return err
		}
		if len(args) > 0 {
			if err := apply(args); err != nil {
				return err
			}
		}
		good = cr.n - int64(br.Buffered())
	}
	return a.truncate(good)
}

// Record runs fn while holding the rewrite barrier. When fn reports that the
// command changed the keyspace, args are appended.
func (a *AOF) Record(args []string, fn func() bool) {
	a.op.RLock()
	defer a.op.RUnlock()
	if !fn() {
		return
	}
	if err := a.append(args); err != nil {
		a.logErr("aof append", err)
	}
}

// LogExpire appends a DEL for a key removed by expiry. The caller holds the
// key's shard lock, so this must not touch the store.
func (a *AOF) LogExpire(key string) {
	if err := a.append([]string{"DEL", key}); err != nil {
		a.logErr("aof append", err)
	}
}

// BeginRewrite starts a background compaction. Wait receives its result.
func (a *AOF) BeginRewrite() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.running {
		return ErrRewriteBusy
	}
	if a.snap == nil {
		return errors.New("aof: no snapshot source")
	}
	a.running = true
	ch := make(chan error, 1)
	a.notify = ch
	a.wg.Add(1)
	go func() {
		defer a.wg.Done()
		err := a.Rewrite()
		a.mu.Lock()
		a.running = false
		a.mu.Unlock()
		ch <- err
		if err != nil {
			a.logErr("aof rewrite", err)
		}
	}()
	return nil
}

// Wait blocks until the rewrite started by BeginRewrite finishes.
func (a *AOF) Wait() error {
	a.mu.Lock()
	ch := a.notify
	a.mu.Unlock()
	if ch == nil {
		return nil
	}
	return <-ch
}

// Close syncs and closes the file. It waits for an in-flight rewrite so the
// process does not exit with the log half-replaced.
func (a *AOF) Close() error {
	a.cancel()
	<-a.stopped
	a.wg.Wait()
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.f == nil {
		return nil
	}
	err := a.f.Sync()
	if cerr := a.f.Close(); err == nil {
		err = cerr
	}
	a.f = nil
	return err
}

func (a *AOF) append(args []string) error {
	payload, err := encode(args)
	if err != nil {
		return err
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.f == nil {
		return errors.New("aof: closed")
	}
	if err := writeFull(a.f, payload); err != nil {
		return err
	}
	if a.rewriting {
		a.diff.Write(payload)
	}
	if a.policy == FsyncAlways {
		return a.f.Sync()
	}
	a.dirty = true
	return nil
}

func (a *AOF) fsyncLoop(ctx context.Context) {
	defer close(a.stopped)
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			a.mu.Lock()
			if a.policy == FsyncEverySec && a.dirty && a.f != nil {
				if err := a.f.Sync(); err != nil {
					a.logErr("aof sync", err)
				} else {
					a.dirty = false
				}
			}
			a.mu.Unlock()
		}
	}
}

func (a *AOF) truncate(n int64) error {
	if err := a.f.Truncate(n); err != nil {
		return err
	}
	_, err := a.f.Seek(n, io.SeekStart)
	return err
}

func (a *AOF) abortRewrite() {
	a.mu.Lock()
	a.rewriting = false
	a.diff.Reset()
	a.mu.Unlock()
}

func (a *AOF) logErr(msg string, err error) {
	if a.logger != nil && err != nil {
		a.logger.Error(msg, "err", err)
	}
}

func encode(args []string) ([]byte, error) {
	if len(args) == 0 {
		return nil, errors.New("aof: empty command")
	}
	parts := make([]resp.Value, len(args))
	for i, arg := range args {
		parts[i] = resp.BulkString(arg)
	}
	var buf bytes.Buffer
	w := resp.NewWriter(&buf)
	if err := w.Write(resp.Array(parts...)); err != nil {
		return nil, err
	}
	if err := w.Flush(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func commandArgs(v resp.Value) ([]string, error) {
	if v.Type != resp.TypeArray || v.Null {
		return nil, errors.New("aof: expected a command array")
	}
	args := make([]string, len(v.Array))
	for i, el := range v.Array {
		if el.Type != resp.TypeBulkString || el.Null {
			return nil, errors.New("aof: expected bulk string arguments")
		}
		args[i] = el.Str
	}
	return args, nil
}

func writeFull(w io.Writer, b []byte) error {
	for len(b) > 0 {
		n, err := w.Write(b)
		b = b[n:]
		if err != nil {
			return err
		}
		if n == 0 {
			return io.ErrShortWrite
		}
	}
	return nil
}

type countingReader struct {
	r io.Reader
	n int64
}

func (c *countingReader) Read(p []byte) (int, error) {
	n, err := c.r.Read(p)
	c.n += int64(n)
	return n, err
}

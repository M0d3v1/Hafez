package persistence

import (
	"bytes"
	"errors"
	"io"
	"os"
	"strconv"

	"github.com/M0d3v1/hafez/internal/store"
)

var errNoSnapshot = errors.New("aof: no snapshot source")

// Rewrite replaces the log with commands that rebuild the current keyspace.
// Clients are paused only while the keyspace is copied. The new file is
// written alongside the live log, and commands that arrive during that write
// are appended before the new file takes its place.
func (a *AOF) Rewrite() error {
	if a.snap == nil {
		return errNoSnapshot
	}
	var snap []store.KeyState
	a.op.Lock()
	a.mu.Lock()
	a.rewriting = true
	a.diff.Reset()
	a.mu.Unlock()
	snap = a.snap.Snapshot()
	a.op.Unlock()

	tmpPath := a.path + ".rewrite"
	tmp, err := os.OpenFile(tmpPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		a.abortRewrite()
		return err
	}
	var buf bytes.Buffer
	for _, key := range snap {
		for _, cmd := range commandsFor(key) {
			payload, err := encode(cmd)
			if err != nil {
				tmp.Close()
				os.Remove(tmpPath)
				a.abortRewrite()
				return err
			}
			buf.Write(payload)
		}
	}
	if err := writeFull(tmp, buf.Bytes()); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		a.abortRewrite()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		a.abortRewrite()
		return err
	}
	return a.install(tmp, tmpPath)
}

func (a *AOF) install(tmp *os.File, tmpPath string) error {
	discard := func(err error) error {
		tmp.Close()
		os.Remove(tmpPath)
		a.abortRewrite()
		return err
	}

	a.mu.Lock()
	for a.diff.Len() > 0 {
		chunk := append([]byte(nil), a.diff.Bytes()...)
		a.diff.Reset()
		a.mu.Unlock()
		werr := writeFull(tmp, chunk)
		a.mu.Lock()
		if werr != nil {
			a.mu.Unlock()
			return discard(werr)
		}
	}
	if err := tmp.Sync(); err != nil {
		a.mu.Unlock()
		return discard(err)
	}
	if err := tmp.Close(); err != nil {
		a.mu.Unlock()
		return discard(err)
	}
	if err := os.Rename(tmpPath, a.path); err != nil {
		a.mu.Unlock()
		os.Remove(tmpPath)
		a.abortRewrite()
		return err
	}
	nf, err := os.OpenFile(a.path, os.O_RDWR, 0o644)
	if err != nil {
		a.rewriting = false
		a.mu.Unlock()
		return err
	}
	if _, err := nf.Seek(0, io.SeekEnd); err != nil {
		nf.Close()
		a.rewriting = false
		a.mu.Unlock()
		return err
	}
	old := a.f
	a.f = nf
	a.rewriting = false
	a.diff.Reset()
	a.dirty = false
	a.mu.Unlock()
	if old != nil {
		old.Close()
	}
	return nil
}

func commandsFor(k store.KeyState) [][]string {
	var cmds [][]string
	switch k.Type {
	case store.TypeString:
		cmds = append(cmds, []string{"SET", k.Key, k.Value})
	case store.TypeList:
		if len(k.List) == 0 {
			return nil
		}
		cmds = append(cmds, append([]string{"RPUSH", k.Key}, k.List...))
	case store.TypeHash:
		if len(k.Fields) == 0 {
			return nil
		}
		cmd := make([]string, 0, 2+len(k.Fields)*2)
		cmd = append(cmd, "HSET", k.Key)
		for _, f := range k.Fields {
			cmd = append(cmd, f.Name, f.Value)
		}
		cmds = append(cmds, cmd)
	default:
		return nil
	}
	if k.HasTTL {
		ms := k.TTL.Milliseconds()
		if ms < 1 {
			ms = 1
		}
		cmds = append(cmds, []string{"PEXPIRE", k.Key, strconv.FormatInt(ms, 10)})
	}
	return cmds
}

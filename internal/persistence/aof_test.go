package persistence

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/M0d3v1/hafez/internal/store"
)

func TestParsePolicy(t *testing.T) {
	always, err := ParsePolicy("ALWAYS")
	if err != nil || always != FsyncAlways {
		t.Fatalf("always = %v %v", always, err)
	}
	if _, err := ParsePolicy("sometimes"); err == nil {
		t.Fatal("accepted an unknown policy")
	}
}

func TestLoadTruncatesPartialTail(t *testing.T) {
	path := filepath.Join(t.TempDir(), "appendonly.aof")
	full, err := encode([]string{"SET", "a", "1"})
	if err != nil {
		t.Fatal(err)
	}
	partial := append(append([]byte{}, full...), []byte("*3\r\n$3\r\nSET\r\n$1\r\nb\r\n$1\r\n")...)
	if err := os.WriteFile(path, partial, 0o644); err != nil {
		t.Fatal(err)
	}

	aof := openAOF(t, path, store.New())
	var got [][]string
	if err := aof.Load(func(args []string) error {
		got = append(got, append([]string(nil), args...))
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0][0] != "SET" || got[0][2] != "1" {
		t.Fatalf("replayed %v", got)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(data, full) {
		t.Fatalf("file = %q, want the complete command only", data)
	}

	aof.Record([]string{"SET", "c", "3"}, func() bool { return true })
	if err := aof.Close(); err != nil {
		t.Fatal(err)
	}
	aof = openAOF(t, path, store.New())
	got = nil
	if err := aof.Load(func(args []string) error {
		got = append(got, append([]string(nil), args...))
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[1][1] != "c" {
		t.Fatalf("after append %v", got)
	}
}

func TestLoadRejectsCompleteGarbage(t *testing.T) {
	path := filepath.Join(t.TempDir(), "appendonly.aof")
	body := []byte("*1\r\n$3\r\nSET\r\n$-2\r\n")
	if err := os.WriteFile(path, body, 0o644); err != nil {
		t.Fatal(err)
	}
	aof := openAOF(t, path, store.New())
	err := aof.Load(func(args []string) error { return nil })
	if err == nil {
		t.Fatal("expected a protocol error")
	}
	data, readErr := os.ReadFile(path)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if !bytes.Equal(data, body) {
		t.Fatal("a protocol error truncated the file")
	}
}

func TestRewriteCompacts(t *testing.T) {
	start := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	now := start
	st := store.New(store.WithClock(func() time.Time { return now }))
	path := filepath.Join(t.TempDir(), "appendonly.aof")
	aof := openAOF(t, path, st)
	if err := aof.Load(func([]string) error { return nil }); err != nil {
		t.Fatal(err)
	}
	aof.Record([]string{"SET", "a", "old"}, func() bool {
		st.Set("a", "old", store.SetOptions{})
		return true
	})
	aof.Record([]string{"SET", "a", "new"}, func() bool {
		st.Set("a", "new", store.SetOptions{})
		return true
	})
	st.Expire("a", 5*time.Second)
	if _, err := st.RPush("l", []string{"x", "y"}); err != nil {
		t.Fatal(err)
	}
	if _, err := st.HSet("h", []store.Field{{Name: "f", Value: "v"}}); err != nil {
		t.Fatal(err)
	}
	if err := aof.Rewrite(); err != nil {
		t.Fatal(err)
	}
	if err := aof.Close(); err != nil {
		t.Fatal(err)
	}

	var got [][]string
	again := openAOF(t, path, store.New())
	if err := again.Load(func(args []string) error {
		got = append(got, append([]string(nil), args...))
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	want := map[string][]string{
		"SET a":     {"SET", "a", "new"},
		"PEXPIRE a": {"PEXPIRE", "a", "5000"},
		"RPUSH l":   {"RPUSH", "l", "x", "y"},
		"HSET h":    {"HSET", "h", "f", "v"},
	}
	if len(got) != len(want) {
		t.Fatalf("commands = %v", got)
	}
	for _, cmd := range got {
		key := cmd[0] + " " + cmd[1]
		exp, ok := want[key]
		if !ok || len(cmd) != len(exp) {
			t.Fatalf("unexpected %v", cmd)
		}
		for i := range exp {
			if cmd[i] != exp[i] {
				t.Fatalf("command %v, want %v", cmd, exp)
			}
		}
		delete(want, key)
	}
}

func TestRewriteBusy(t *testing.T) {
	aof := openAOF(t, filepath.Join(t.TempDir(), "appendonly.aof"), store.New())
	aof.op.Lock()
	if err := aof.BeginRewrite(); err != nil {
		aof.op.Unlock()
		t.Fatal(err)
	}
	err := aof.BeginRewrite()
	aof.op.Unlock()
	if !errors.Is(err, ErrRewriteBusy) {
		t.Fatalf("second rewrite = %v", err)
	}
	if err := aof.Wait(); err != nil {
		t.Fatal(err)
	}
}

func TestCloseStopsFsync(t *testing.T) {
	aof := openAOF(t, filepath.Join(t.TempDir(), "appendonly.aof"), store.New())
	done := make(chan error, 1)
	go func() { done <- aof.Close() }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("close did not stop the sync loop")
	}
}

func openAOF(t *testing.T, path string, st *store.Memory) *AOF {
	t.Helper()
	aof, err := Open(path, FsyncAlways, st, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { aof.Close() })
	return aof
}

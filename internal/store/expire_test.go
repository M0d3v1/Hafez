package store

import (
	"context"
	"strconv"
	"testing"
	"time"
)

func TestExpirePersistTTL(t *testing.T) {
	start := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	now := start
	m := New(WithClock(func() time.Time { return now }))

	if m.Expire("missing", time.Second) {
		t.Fatal("expire on a missing key")
	}
	m.Set("k", "v", SetOptions{})
	if !m.Expire("k", 1500*time.Millisecond) {
		t.Fatal("expire")
	}
	rem, exists, expires := m.TTL("k")
	if !exists || !expires || rem != 1500*time.Millisecond {
		t.Fatalf("ttl = %s exists=%v expires=%v", rem, exists, expires)
	}
	if !m.Persist("k") {
		t.Fatal("persist")
	}
	if _, exists, expires = m.TTL("k"); !exists || expires {
		t.Fatalf("after persist exists=%v expires=%v", exists, expires)
	}
	if m.Persist("k") {
		t.Fatal("persist twice")
	}

	m.Expire("k", time.Second)
	now = start.Add(time.Second)
	if _, exists, _ = m.TTL("k"); exists {
		t.Fatal("ttl should drop an expired key")
	}
	if _, ok, _ := m.Get("k"); ok {
		t.Fatal("key still present")
	}
}

func TestExpireNonPositiveDeletes(t *testing.T) {
	m := New()
	m.Set("k", "v", SetOptions{})
	if !m.Expire("k", 0) {
		t.Fatal("expire 0")
	}
	if _, ok, _ := m.Get("k"); ok {
		t.Fatal("key survived expire 0")
	}
	m.Set("k", "v", SetOptions{})
	if !m.Expire("k", -time.Second) {
		t.Fatal("negative expire")
	}
	if m.DBSize() != 0 {
		t.Fatal("dbsize")
	}
}

func TestActiveExpireCycle(t *testing.T) {
	start := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	now := start
	m := New(WithClock(func() time.Time { return now }))
	for i := 0; i < 10; i++ {
		m.Set(strconv.Itoa(i), "v", SetOptions{TTL: time.Second, HasTTL: true})
	}
	m.Set("keep", "v", SetOptions{TTL: time.Hour, HasTTL: true})
	m.Set("plain", "v", SetOptions{})
	now = start.Add(time.Second)

	if m.DBSize() != 12 {
		t.Fatalf("dbsize before cycle = %d", m.DBSize())
	}
	if deleted := m.ActiveExpireCycle(); deleted != 10 {
		t.Fatalf("deleted %d", deleted)
	}
	if _, ok, _ := m.Get("keep"); !ok {
		t.Fatal("future key was removed")
	}
	if _, ok, _ := m.Get("plain"); !ok {
		t.Fatal("persistent key was removed")
	}
	if _, ok, _ := m.Get("0"); ok {
		t.Fatal("expired key survived the cycle")
	}
	if m.DBSize() != 2 {
		t.Fatalf("dbsize after cycle = %d", m.DBSize())
	}
}

func TestOnExpire(t *testing.T) {
	start := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	now := start
	m := New(WithClock(func() time.Time { return now }))
	var got []string
	m.OnExpire(func(key string) { got = append(got, key) })
	m.Set("k", "v", SetOptions{})
	m.Expire("k", time.Second)
	m.Set("other", "v", SetOptions{TTL: time.Second, HasTTL: true})
	now = start.Add(time.Second)
	if _, ok, _ := m.Get("k"); ok {
		t.Fatal("lazy expire left the key")
	}
	if deleted := m.ActiveExpireCycle(); deleted != 1 {
		t.Fatalf("deleted %d", deleted)
	}
	if len(got) != 2 || got[0] != "k" || got[1] != "other" {
		t.Fatalf("expired = %v", got)
	}
}

func TestRunActiveExpireStops(t *testing.T) {
	m := New()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		m.RunActiveExpire(ctx)
		close(done)
	}()
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("expire loop did not stop")
	}
}

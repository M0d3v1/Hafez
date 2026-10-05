package store

import (
	"errors"
	"math"
	"sort"
	"strconv"
	"sync"
	"testing"
	"time"
)

func TestSetGetDelExists(t *testing.T) {
	m := New()
	if _, ok, err := m.Get("missing"); err != nil || ok {
		t.Fatalf("missing get ok=%v err=%v", ok, err)
	}
	if !m.Set("a", "1", SetOptions{}) {
		t.Fatal("set")
	}
	if m.Set("a", "2", SetOptions{NX: true}) {
		t.Fatal("nx should reject")
	}
	got, ok, err := m.Get("a")
	if err != nil || !ok || got != "1" {
		t.Fatalf("get after nx = %q %v %v", got, ok, err)
	}
	if m.Set("missing", "x", SetOptions{XX: true}) {
		t.Fatal("xx should reject a missing key")
	}
	if !m.Set("a", "2", SetOptions{XX: true}) {
		t.Fatal("xx should replace")
	}
	if m.Exists([]string{"a", "a", "nope"}) != 2 {
		t.Fatalf("exists = %d", m.Exists([]string{"a", "a", "nope"}))
	}
	if m.Del([]string{"a", "a", "nope"}) != 1 {
		t.Fatal("del count")
	}
	if m.Exists([]string{"a"}) != 0 {
		t.Fatal("exists after del")
	}
}

func TestExpiryIsLazy(t *testing.T) {
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	now := start
	m := New(WithClock(func() time.Time { return now }))

	if !m.Set("k", "v", SetOptions{TTL: 5 * time.Second, HasTTL: true}) {
		t.Fatal("set")
	}
	if _, ok, _ := m.Get("k"); !ok {
		t.Fatal("expected the key before the deadline")
	}
	now = start.Add(5 * time.Second)
	if m.DBSize() != 1 {
		t.Fatalf("dbsize before touch = %d", m.DBSize())
	}
	if _, ok, err := m.Get("k"); err != nil || ok {
		t.Fatalf("get after deadline ok=%v err=%v", ok, err)
	}
	if m.DBSize() != 0 {
		t.Fatalf("dbsize after touch = %d", m.DBSize())
	}
}

func TestKeysDropsExpired(t *testing.T) {
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	now := start
	m := New(WithClock(func() time.Time { return now }))
	m.Set("k", "v", SetOptions{TTL: time.Second, HasTTL: true})
	m.Set("keep", "v", SetOptions{})
	now = start.Add(time.Second)
	got := m.Keys("*")
	if len(got) != 1 || got[0] != "keep" {
		t.Fatalf("keys = %v", got)
	}
	if m.DBSize() != 1 {
		t.Fatalf("dbsize = %d", m.DBSize())
	}
}

func TestIncrBy(t *testing.T) {
	m := New()
	n, err := m.IncrBy("c", 1)
	if err != nil || n != 1 {
		t.Fatalf("incr missing = %d %v", n, err)
	}
	n, err = m.IncrBy("c", -3)
	if err != nil || n != -2 {
		t.Fatalf("incr = %d %v", n, err)
	}
	m.Set("bad", "hello", SetOptions{})
	if _, err := m.IncrBy("bad", 1); !errors.Is(err, ErrNotInteger) {
		t.Fatalf("err = %v", err)
	}
	m.Set("max", strconv.FormatInt(math.MaxInt64, 10), SetOptions{})
	if _, err := m.IncrBy("max", 1); !errors.Is(err, ErrNotInteger) {
		t.Fatalf("overflow err = %v", err)
	}
	got, _, _ := m.Get("max")
	if got != strconv.FormatInt(math.MaxInt64, 10) {
		t.Fatalf("overflow changed the value to %q", got)
	}

	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	now := start
	timed := New(WithClock(func() time.Time { return now }))
	timed.Set("n", "1", SetOptions{TTL: time.Second, HasTTL: true})
	if _, err := timed.IncrBy("n", 1); err != nil {
		t.Fatal(err)
	}
	now = start.Add(time.Second)
	if _, ok, _ := timed.Get("n"); ok {
		t.Fatal("incr should have kept the deadline")
	}
}

func TestMSetMGetKeysTypeFlush(t *testing.T) {
	m := New()
	m.MSet([]Pair{{Key: "b", Value: "2"}, {Key: "a", Value: "1"}, {Key: "a", Value: "9"}})
	items, err := m.MGet([]string{"a", "missing", "b"})
	if err != nil {
		t.Fatal(err)
	}
	if items[0].Value != "9" || !items[1].Null || items[2].Value != "2" {
		t.Fatalf("%#v", items)
	}
	if m.Type("a") != TypeString || m.Type("missing") != TypeNone {
		t.Fatalf("types %q %q", m.Type("a"), m.Type("missing"))
	}
	got := m.Keys("a*")
	sort.Strings(got)
	if len(got) != 1 || got[0] != "a" {
		t.Fatalf("keys %v", got)
	}
	if m.DBSize() != 2 {
		t.Fatalf("dbsize %d", m.DBSize())
	}
	m.FlushAll()
	if m.DBSize() != 0 {
		t.Fatalf("dbsize after flush %d", m.DBSize())
	}
}

func TestWrongType(t *testing.T) {
	m := New()
	s := m.shard("k")
	s.mu.Lock()
	s.data["k"] = entry{typ: "list", str: "x"}
	s.mu.Unlock()

	if _, _, err := m.Get("k"); !errors.Is(err, ErrWrongType) {
		t.Fatalf("get err %v", err)
	}
	if _, err := m.IncrBy("k", 1); !errors.Is(err, ErrWrongType) {
		t.Fatalf("incr err %v", err)
	}
	if _, err := m.MGet([]string{"k"}); !errors.Is(err, ErrWrongType) {
		t.Fatalf("mget err %v", err)
	}
	if m.Type("k") != "list" {
		t.Fatalf("type %q", m.Type("k"))
	}
	if m.Del([]string{"k"}) != 1 {
		t.Fatal("del should remove any type")
	}
}

func TestConcurrentIncr(t *testing.T) {
	m := New()
	const goroutines = 8
	const each = 40
	var wg sync.WaitGroup
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < each; j++ {
				if _, err := m.IncrBy("c", 1); err != nil {
					t.Error(err)
					return
				}
			}
		}()
	}
	wg.Wait()
	got, ok, err := m.Get("c")
	if err != nil || !ok || got != strconv.Itoa(goroutines*each) {
		t.Fatalf("got %q ok=%v err=%v", got, ok, err)
	}
}

func TestShardSpread(t *testing.T) {
	seen := map[int]bool{}
	for i := 0; i < 1000; i++ {
		seen[shardIndex(strconv.Itoa(i))] = true
	}
	if len(seen) < ShardCount/2 {
		t.Fatalf("only %d shards used", len(seen))
	}
}

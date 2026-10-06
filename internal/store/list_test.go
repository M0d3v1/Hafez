package store

import (
	"errors"
	"reflect"
	"sync"
	"testing"
	"time"
)

func TestListPushPopRange(t *testing.T) {
	m := New()
	n, err := m.LPush("l", []string{"a", "b", "c"})
	if err != nil || n != 3 {
		t.Fatalf("lpush %d %v", n, err)
	}
	got, err := m.LRange("l", 0, -1)
	if err != nil || !reflect.DeepEqual(got, []string{"c", "b", "a"}) {
		t.Fatalf("after lpush %v %v", got, err)
	}
	n, err = m.RPush("l", []string{"d"})
	if err != nil || n != 4 {
		t.Fatalf("rpush %d %v", n, err)
	}
	got, err = m.LRange("l", -2, -1)
	if err != nil || !reflect.DeepEqual(got, []string{"a", "d"}) {
		t.Fatalf("tail %v %v", got, err)
	}
	got, err = m.LRange("l", 5, 10)
	if err != nil || len(got) != 0 {
		t.Fatalf("out of range %v %v", got, err)
	}
	if n, err = m.LLen("missing"); err != nil || n != 0 {
		t.Fatalf("llen missing %d %v", n, err)
	}
	if n, err = m.LLen("l"); err != nil || n != 4 {
		t.Fatalf("llen %d %v", n, err)
	}

	v, ok, err := m.LPop("l")
	if err != nil || !ok || v != "c" {
		t.Fatalf("lpop %q %v %v", v, ok, err)
	}
	v, ok, err = m.RPop("l")
	if err != nil || !ok || v != "d" {
		t.Fatalf("rpop %q %v %v", v, ok, err)
	}
	if _, ok, err = m.LPop("missing"); err != nil || ok {
		t.Fatalf("lpop missing ok=%v err=%v", ok, err)
	}

	m.LPop("l")
	m.LPop("l")
	if m.DBSize() != 0 {
		t.Fatalf("empty list should drop the key, dbsize %d", m.DBSize())
	}
	if _, ok, _ = m.LPop("l"); ok {
		t.Fatal("pop after empty")
	}
}

func TestListWrongTypeAndExpiry(t *testing.T) {
	m := New()
	m.Set("s", "v", SetOptions{})
	if _, err := m.LPush("s", []string{"a"}); !errors.Is(err, ErrWrongType) {
		t.Fatalf("lpush string %v", err)
	}
	m.LPush("l", []string{"a"})
	if _, _, err := m.Get("l"); !errors.Is(err, ErrWrongType) {
		t.Fatalf("get list %v", err)
	}
	if m.Type("l") != TypeList {
		t.Fatalf("type %q", m.Type("l"))
	}

	start := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	now := start
	timed := New(WithClock(func() time.Time { return now }))
	timed.LPush("l", []string{"a"})
	if !timed.Expire("l", time.Second) {
		t.Fatal("expire")
	}
	timed.RPush("l", []string{"b"})
	rem, exists, expires := timed.TTL("l")
	if !exists || !expires || rem != time.Second {
		t.Fatalf("push cleared ttl %s %v %v", rem, exists, expires)
	}
	now = start.Add(time.Second)
	if n, err := timed.LLen("l"); err != nil || n != 0 {
		t.Fatalf("llen expired %d %v", n, err)
	}
}

func TestListConcurrentPush(t *testing.T) {
	m := New()
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 20; j++ {
				if _, err := m.RPush("l", []string{"x"}); err != nil {
					t.Error(err)
					return
				}
			}
		}()
	}
	wg.Wait()
	if n, err := m.LLen("l"); err != nil || n != 160 {
		t.Fatalf("llen %d %v", n, err)
	}
}

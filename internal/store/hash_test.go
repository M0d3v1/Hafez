package store

import (
	"errors"
	"reflect"
	"testing"
	"time"
)

func TestHashCRUD(t *testing.T) {
	m := New()
	added, err := m.HSet("h", []Field{{"a", "1"}, {"b", "2"}, {"a", "3"}})
	if err != nil || added != 2 {
		t.Fatalf("hset %d %v", added, err)
	}
	v, ok, err := m.HGet("h", "a")
	if err != nil || !ok || v != "3" {
		t.Fatalf("hget %q %v %v", v, ok, err)
	}
	if _, ok, err = m.HGet("h", "missing"); err != nil || ok {
		t.Fatalf("missing field ok=%v err=%v", ok, err)
	}
	exists, err := m.HExists("h", "b")
	if err != nil || !exists {
		t.Fatalf("hexists %v %v", exists, err)
	}
	if n, err := m.HLen("h"); err != nil || n != 2 {
		t.Fatalf("hlen %d %v", n, err)
	}
	got, err := m.HGetAll("h")
	if err != nil || !reflect.DeepEqual(got, []Field{{"a", "3"}, {"b", "2"}}) {
		t.Fatalf("hgetall %#v %v", got, err)
	}

	n, err := m.HDel("h", []string{"b", "missing", "b"})
	if err != nil || n != 1 {
		t.Fatalf("hdel %d %v", n, err)
	}
	got, err = m.HGetAll("h")
	if err != nil || !reflect.DeepEqual(got, []Field{{"a", "3"}}) {
		t.Fatalf("after hdel %#v %v", got, err)
	}
	if n, err = m.HDel("h", []string{"a"}); err != nil || n != 1 {
		t.Fatalf("hdel last %d %v", n, err)
	}
	if m.DBSize() != 0 {
		t.Fatalf("empty hash should drop the key, dbsize %d", m.DBSize())
	}
	if n, err = m.HLen("missing"); err != nil || n != 0 {
		t.Fatalf("hlen missing %d %v", n, err)
	}
	if got, err = m.HGetAll("missing"); err != nil || len(got) != 0 {
		t.Fatalf("hgetall missing %#v %v", got, err)
	}
}

func TestHashWrongType(t *testing.T) {
	m := New()
	m.Set("s", "v", SetOptions{})
	if _, err := m.HSet("s", []Field{{"a", "1"}}); !errors.Is(err, ErrWrongType) {
		t.Fatalf("hset string %v", err)
	}
	m.HSet("h", []Field{{"a", "1"}})
	if _, _, err := m.Get("h"); !errors.Is(err, ErrWrongType) {
		t.Fatalf("get hash %v", err)
	}
	if _, err := m.LPush("h", []string{"x"}); !errors.Is(err, ErrWrongType) {
		t.Fatalf("lpush hash %v", err)
	}
	if m.Type("h") != TypeHash {
		t.Fatalf("type %q", m.Type("h"))
	}

	start := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	now := start
	timed := New(WithClock(func() time.Time { return now }))
	timed.HSet("h", []Field{{"a", "1"}})
	timed.Expire("h", time.Second)
	now = start.Add(time.Second)
	if _, ok, err := timed.HGet("h", "a"); err != nil || ok {
		t.Fatalf("hget expired ok=%v err=%v", ok, err)
	}
}

package store

import (
	"testing"
	"time"
)

func TestSnapshotCopiesLiveKeys(t *testing.T) {
	start := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	now := start
	m := New(WithClock(func() time.Time { return now }))
	m.Set("s", "v", SetOptions{})
	m.Expire("s", 5*time.Second)
	if _, err := m.RPush("l", []string{"a", "b"}); err != nil {
		t.Fatal(err)
	}
	if _, err := m.HSet("h", []Field{{Name: "f", Value: "1"}, {Name: "g", Value: "2"}}); err != nil {
		t.Fatal(err)
	}
	m.Set("gone", "x", SetOptions{TTL: time.Second, HasTTL: true})
	now = start.Add(time.Second)

	snap := m.Snapshot()
	byKey := map[string]KeyState{}
	for _, k := range snap {
		byKey[k.Key] = k
	}
	if _, ok := byKey["gone"]; ok {
		t.Fatal("snapshot kept an expired key")
	}
	s := byKey["s"]
	if s.Type != TypeString || s.Value != "v" || !s.HasTTL || s.TTL != 4*time.Second {
		t.Fatalf("string snapshot = %+v", s)
	}
	l := byKey["l"]
	if l.Type != TypeList || len(l.List) != 2 || l.List[0] != "a" || l.List[1] != "b" {
		t.Fatalf("list snapshot = %+v", l)
	}
	h := byKey["h"]
	if h.Type != TypeHash || len(h.Fields) != 2 || h.Fields[0].Name != "f" || h.Fields[1].Value != "2" {
		t.Fatalf("hash snapshot = %+v", h)
	}

	m.Set("s", "changed", SetOptions{})
	if s.Value != "v" {
		t.Fatal("snapshot shares storage with the live key")
	}
	m.RPush("l", []string{"c"})
	if len(l.List) != 2 {
		t.Fatal("snapshot shares the list")
	}
}

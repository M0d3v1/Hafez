package store

import (
	"strconv"
	"testing"
)

func BenchmarkSetGet(b *testing.B) {
	m := New()
	keys := make([]string, 1024)
	for i := range keys {
		keys[i] = strconv.Itoa(i)
		m.Set(keys[i], "value", SetOptions{})
	}
	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			key := keys[i%len(keys)]
			if _, _, err := m.Get(key); err != nil {
				b.Fatal(err)
			}
			m.Set(key, "value", SetOptions{})
			i++
		}
	})
}

func BenchmarkIncr(b *testing.B) {
	m := New()
	keys := make([]string, 1024)
	for i := range keys {
		keys[i] = strconv.Itoa(i)
		m.Set(keys[i], "0", SetOptions{})
	}
	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			if _, err := m.IncrBy(keys[i%len(keys)], 1); err != nil {
				b.Fatal(err)
			}
			i++
		}
	})
}

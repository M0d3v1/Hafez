package resp

import (
	"bytes"
	"testing"
)

func BenchmarkReadCommand(b *testing.B) {
	raw := []byte("*2\r\n$3\r\nGET\r\n$3\r\nkey\r\n")
	blob := bytes.Repeat(raw, b.N)
	rd := NewReader(bytes.NewReader(blob))
	b.ReportAllocs()
	b.SetBytes(int64(len(raw)))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		v, err := rd.Read()
		if err != nil || v.Type != TypeArray {
			b.Fatalf("read: %v", err)
		}
	}
}

func BenchmarkWriteCommand(b *testing.B) {
	v := Array(BulkString("SET"), BulkString("key"), BulkString("value"))
	var buf bytes.Buffer
	w := NewWriter(&buf)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf.Reset()
		if err := w.Write(v); err != nil {
			b.Fatal(err)
		}
		if err := w.Flush(); err != nil {
			b.Fatal(err)
		}
	}
}

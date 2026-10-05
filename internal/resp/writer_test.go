package resp

import (
	"bytes"
	"reflect"
	"testing"
)

func TestWriteValues(t *testing.T) {
	tests := []struct {
		name string
		v    Value
		want string
	}{
		{name: "simple", v: SimpleString("OK"), want: "+OK\r\n"},
		{name: "empty simple", v: SimpleString(""), want: "+\r\n"},
		{name: "error", v: Error("ERR no"), want: "-ERR no\r\n"},
		{name: "integer", v: Integer(42), want: ":42\r\n"},
		{name: "negative", v: Integer(-7), want: ":-7\r\n"},
		{name: "bulk", v: BulkString("hi"), want: "$2\r\nhi\r\n"},
		{name: "empty bulk", v: BulkString(""), want: "$0\r\n\r\n"},
		{name: "null bulk", v: NullBulk(), want: "$-1\r\n"},
		{name: "empty array", v: Array(), want: "*0\r\n"},
		{name: "null array", v: NullArray(), want: "*-1\r\n"},
		{
			name: "array",
			v:    Array(BulkString("PING"), Integer(1), NullBulk()),
			want: "*3\r\n$4\r\nPING\r\n:1\r\n$-1\r\n",
		},
		{
			name: "nested",
			v:    Array(Array(SimpleString("OK"))),
			want: "*1\r\n*1\r\n+OK\r\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			w := NewWriter(&buf)
			if err := w.Write(tt.v); err != nil {
				t.Fatal(err)
			}
			if buf.Len() != 0 {
				t.Fatal("Write flushed early")
			}
			if err := w.Flush(); err != nil {
				t.Fatal(err)
			}
			if buf.String() != tt.want {
				t.Fatalf("encoded %q, want %q", buf.String(), tt.want)
			}
		})
	}
}

func TestWriteRejectsBrokenSimpleString(t *testing.T) {
	var buf bytes.Buffer
	w := NewWriter(&buf)
	if err := w.Write(SimpleString("bad\r\n")); err == nil {
		t.Fatal("expected error for CR/LF in a simple string")
	}
	if err := w.Write(Value{}); err == nil {
		t.Fatal("expected error for unknown type")
	}
}

func TestRoundTrip(t *testing.T) {
	values := []Value{
		SimpleString("PONG"),
		Error("ERR unknown command 'GET'"),
		Integer(0),
		Integer(-1),
		BulkString(""),
		BulkString("hello world"),
		BulkString("a\x00b"),
		NullBulk(),
		Array(),
		NullArray(),
		Array(BulkString("ECHO"), BulkString("x")),
		Array(Array(Integer(1), NullBulk()), SimpleString("OK")),
	}
	for _, v := range values {
		var buf bytes.Buffer
		w := NewWriter(&buf)
		if err := w.Write(v); err != nil {
			t.Fatal(err)
		}
		if err := w.Flush(); err != nil {
			t.Fatal(err)
		}
		got, err := NewReader(bytes.NewReader(buf.Bytes())).Read()
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(got, v) {
			t.Fatalf("round trip %#v -> %q -> %#v", v, buf.String(), got)
		}
	}
}

package resp

import (
	"errors"
	"io"
	"reflect"
	"strings"
	"testing"
)

func TestReadValues(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want Value
	}{
		{name: "simple string", in: "+OK\r\n", want: SimpleString("OK")},
		{name: "empty simple string", in: "+\r\n", want: SimpleString("")},
		{name: "error", in: "-ERR unknown command\r\n", want: Error("ERR unknown command")},
		{name: "integer", in: ":123\r\n", want: Integer(123)},
		{name: "negative integer", in: ":-42\r\n", want: Integer(-42)},
		{name: "zero", in: ":0\r\n", want: Integer(0)},
		{name: "bulk", in: "$5\r\nhello\r\n", want: BulkString("hello")},
		{name: "empty bulk", in: "$0\r\n\r\n", want: BulkString("")},
		{name: "bulk with space", in: "$11\r\nhello world\r\n", want: BulkString("hello world")},
		{name: "bulk with null byte", in: "$3\r\na\x00b\r\n", want: BulkString("a\x00b")},
		{name: "null bulk", in: "$-1\r\n", want: NullBulk()},
		{name: "empty array", in: "*0\r\n", want: Array()},
		{name: "null array", in: "*-1\r\n", want: NullArray()},
		{
			name: "array of bulks",
			in:   "*2\r\n$3\r\nGET\r\n$3\r\nkey\r\n",
			want: Array(BulkString("GET"), BulkString("key")),
		},
		{
			name: "nested array",
			in:   "*2\r\n*1\r\n+OK\r\n:7\r\n",
			want: Array(Array(SimpleString("OK")), Integer(7)),
		},
		{
			name: "null bulk element",
			in:   "*1\r\n$-1\r\n",
			want: Array(NullBulk()),
		},
		{name: "inline", in: "PING\r\n", want: Array(BulkString("PING"))},
		{name: "inline lf only", in: "PING\n", want: Array(BulkString("PING"))},
		{name: "inline args", in: "ECHO hello\r\n", want: Array(BulkString("ECHO"), BulkString("hello"))},
		{
			name: "inline double quotes",
			in:   "ECHO \"hello world\"\r\n",
			want: Array(BulkString("ECHO"), BulkString("hello world")),
		},
		{
			name: "inline single quotes",
			in:   "ECHO 'hello world'\r\n",
			want: Array(BulkString("ECHO"), BulkString("hello world")),
		},
		{
			name: "inline escapes",
			in:   "ECHO \"a\\nb\\x41\"\r\n",
			want: Array(BulkString("ECHO"), BulkString("a\nbA")),
		},
		{
			name: "inline quote joins token",
			in:   "foo\"bar\" baz\r\n",
			want: Array(BulkString("foobar"), BulkString("baz")),
		},
		{
			name: "inline empty quotes",
			in:   "ECHO \"\"\r\n",
			want: Array(BulkString("ECHO"), BulkString("")),
		},
		{
			name: "inline single quote escape",
			in:   "ECHO 'it\\'s'\r\n",
			want: Array(BulkString("ECHO"), BulkString("it's")),
		},
		{name: "blank inline line", in: "\r\n", want: Array()},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewReader(strings.NewReader(tt.in)).Read()
			if err != nil {
				t.Fatalf("Read() error = %v", err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("Read() = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestReadMultiple(t *testing.T) {
	r := NewReader(strings.NewReader("+OK\r\n:1\r\n$3\r\nfoo\r\n"))
	first, err := r.Read()
	if err != nil {
		t.Fatal(err)
	}
	second, err := r.Read()
	if err != nil {
		t.Fatal(err)
	}
	third, err := r.Read()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := r.Read(); !errors.Is(err, io.EOF) {
		t.Fatalf("final Read() error = %v, want EOF", err)
	}
	if !reflect.DeepEqual(first, SimpleString("OK")) || second.Int != 1 || third.Str != "foo" {
		t.Fatalf("got %#v %#v %#v", first, second, third)
	}
}

func TestReadMalformed(t *testing.T) {
	tests := []struct {
		name   string
		in     string
		reason string
		eof    error
	}{
		{name: "clean eof", in: "", eof: io.EOF},
		{name: "truncated simple", in: "+OK", eof: io.ErrUnexpectedEOF},
		{name: "truncated after prefix", in: "$", eof: io.ErrUnexpectedEOF},
		{name: "missing cr", in: "+OK\n", reason: "expected CRLF"},
		{name: "bad integer", in: ":nope\r\n", reason: "invalid integer"},
		{name: "huge integer", in: ":999999999999999999999\r\n", reason: "invalid integer"},
		{name: "bad bulk length", in: "$x\r\n", reason: "invalid bulk length"},
		{name: "negative bulk length", in: "$-2\r\n", reason: "invalid bulk length"},
		{name: "truncated bulk", in: "$5\r\nhel", eof: io.ErrUnexpectedEOF},
		{name: "bulk missing crlf", in: "$1\r\nXY\n", reason: "expected CRLF"},
		{name: "bad multibulk length", in: "*x\r\n", reason: "invalid multibulk length"},
		{name: "negative multibulk", in: "*-2\r\n", reason: "invalid multibulk length"},
		{name: "truncated array", in: "*1\r\n", eof: io.ErrUnexpectedEOF},
		{name: "bad element length", in: "*1\r\n$-2\r\n", reason: "invalid bulk length"},
		{name: "inline inside array", in: "*1\r\nPING\r\n", reason: "unknown type byte 'P'"},
		{name: "unbalanced quotes", in: "ECHO \"hello\r\n", reason: "unbalanced quotes in inline command"},
		{name: "junk after quote", in: "ECHO \"foo\"bar\r\n", reason: "unexpected character after quote"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewReader(strings.NewReader(tt.in)).Read()
			if tt.eof != nil {
				if !errors.Is(err, tt.eof) {
					t.Fatalf("Read() error = %v, want %v", err, tt.eof)
				}
				return
			}
			var pe *ProtocolError
			if !errors.As(err, &pe) {
				t.Fatalf("Read() error = %T %v, want ProtocolError", err, err)
			}
			if pe.Reason != tt.reason {
				t.Fatalf("reason = %q, want %q", pe.Reason, tt.reason)
			}
		})
	}
}

func TestReadPartial(t *testing.T) {
	raw := "*2\r\n$4\r\nPING\r\n$5\r\nhello\r\n"
	want := Array(BulkString("PING"), BulkString("hello"))
	for _, n := range []int{1, 2, 3, 5, 8, 64} {
		got, err := NewReader(&chunkReader{data: []byte(raw), n: n}).Read()
		if err != nil {
			t.Fatalf("chunk %d: %v", n, err)
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("chunk %d: got %#v", n, got)
		}
	}
}

func TestReadLimits(t *testing.T) {
	t.Run("bulk too large", func(t *testing.T) {
		r := NewReader(strings.NewReader("$5\r\nhello\r\n"))
		r.maxBulk = 4
		_, err := r.Read()
		assertProtocol(t, err, "bulk string too large")
	})
	t.Run("line too long", func(t *testing.T) {
		r := NewReader(strings.NewReader("+HELLO\r\n"))
		r.maxBulk = 4
		_, err := r.Read()
		assertProtocol(t, err, "line too long")
	})
	t.Run("array too long", func(t *testing.T) {
		r := NewReader(strings.NewReader("*2\r\n$1\r\na\r\n$1\r\nb\r\n"))
		r.maxArray = 1
		_, err := r.Read()
		assertProtocol(t, err, "invalid multibulk length")
	})
	t.Run("nested too deep", func(t *testing.T) {
		r := NewReader(strings.NewReader("*1\r\n$1\r\nx\r\n"))
		r.maxDepth = 0
		_, err := r.Read()
		assertProtocol(t, err, "nested array too deep")
	})
}

func assertProtocol(t *testing.T, err error, reason string) {
	t.Helper()
	var pe *ProtocolError
	if !errors.As(err, &pe) {
		t.Fatalf("error = %v, want ProtocolError %q", err, reason)
	}
	if pe.Reason != reason {
		t.Fatalf("reason = %q, want %q", pe.Reason, reason)
	}
}

type chunkReader struct {
	data []byte
	n    int
	off  int
}

func (c *chunkReader) Read(p []byte) (int, error) {
	if c.off >= len(c.data) || c.n <= 0 {
		return 0, io.EOF
	}
	n := c.n
	if n > len(p) {
		n = len(p)
	}
	if remain := len(c.data) - c.off; n > remain {
		n = remain
	}
	copy(p, c.data[c.off:c.off+n])
	c.off += n
	return n, nil
}

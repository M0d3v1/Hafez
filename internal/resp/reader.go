package resp

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"
)

const (
	defaultMaxArray = 1024 * 1024
	defaultMaxDepth = 128
)

// ProtocolError is a malformed RESP2 frame. The connection that produced
// it should be closed after the error is reported to the client.
type ProtocolError struct {
	Reason string
}

func (e *ProtocolError) Error() string { return e.Reason }

func protocolError(reason string) error {
	return &ProtocolError{Reason: reason}
}

// Reader decodes RESP2 values from an io.Reader. A leading byte that is not
// a type prefix is parsed as one inline command and returned as an array of
// bulk strings.
type Reader struct {
	r        *bufio.Reader
	maxBulk  int64
	maxArray int
	maxDepth int
}

// NewReader returns a Reader that reads RESP2 values from r.
func NewReader(r io.Reader) *Reader {
	br, ok := r.(*bufio.Reader)
	if !ok {
		br = bufio.NewReader(r)
	}
	return &Reader{
		r:        br,
		maxBulk:  MaxBulkSize,
		maxArray: defaultMaxArray,
		maxDepth: defaultMaxDepth,
	}
}

// Read reads the next RESP2 value. A clean end of input before any byte of
// a value returns io.EOF. A closed stream in the middle of a value returns
// io.ErrUnexpectedEOF.
func (rd *Reader) Read() (Value, error) {
	return rd.readValue(true, 0)
}

func (rd *Reader) readValue(allowInline bool, depth int) (Value, error) {
	if depth > rd.maxDepth {
		return Value{}, protocolError("nested array too deep")
	}
	b, err := rd.r.ReadByte()
	if err != nil {
		return Value{}, err
	}
	switch Type(b) {
	case TypeSimpleString, TypeError:
		s, err := rd.readCRLFLine()
		if err != nil {
			return Value{}, err
		}
		return Value{Type: Type(b), Str: s}, nil
	case TypeInteger:
		s, err := rd.readCRLFLine()
		if err != nil {
			return Value{}, err
		}
		n, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			return Value{}, protocolError("invalid integer")
		}
		return Integer(n), nil
	case TypeBulkString:
		return rd.readBulk()
	case TypeArray:
		return rd.readArray(depth)
	default:
		if !allowInline {
			return Value{}, protocolError(fmt.Sprintf("unknown type byte %q", b))
		}
		if err := rd.r.UnreadByte(); err != nil {
			return Value{}, err
		}
		return rd.readInline()
	}
}

func (rd *Reader) readBulk() (Value, error) {
	s, err := rd.readCRLFLine()
	if err != nil {
		return Value{}, eofInValue(err)
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil || n < -1 {
		return Value{}, protocolError("invalid bulk length")
	}
	if n == -1 {
		return NullBulk(), nil
	}
	if n > rd.maxBulk || int64(int(n)) != n {
		return Value{}, protocolError("bulk string too large")
	}
	buf := make([]byte, int(n)+2)
	if _, err := io.ReadFull(rd.r, buf); err != nil {
		return Value{}, eofInValue(err)
	}
	if buf[n] != '\r' || buf[n+1] != '\n' {
		return Value{}, protocolError("expected CRLF")
	}
	return BulkString(string(buf[:n])), nil
}

func (rd *Reader) readArray(depth int) (Value, error) {
	s, err := rd.readCRLFLine()
	if err != nil {
		return Value{}, eofInValue(err)
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil || n < -1 {
		return Value{}, protocolError("invalid multibulk length")
	}
	if n == -1 {
		return NullArray(), nil
	}
	if n > int64(rd.maxArray) {
		return Value{}, protocolError("invalid multibulk length")
	}
	items := make([]Value, 0, int(n))
	for i := int64(0); i < n; i++ {
		el, err := rd.readValue(false, depth+1)
		if err != nil {
			return Value{}, eofInValue(err)
		}
		items = append(items, el)
	}
	return Array(items...), nil
}

func (rd *Reader) readInline() (Value, error) {
	line, err := rd.readInlineLine()
	if err != nil {
		return Value{}, err
	}
	args, err := splitInline(line)
	if err != nil {
		return Value{}, err
	}
	items := make([]Value, len(args))
	for i, arg := range args {
		items[i] = BulkString(arg)
	}
	return Array(items...), nil
}

func (rd *Reader) readCRLFLine() (string, error) {
	s, err := rd.readUntilNewline(true)
	if err != nil {
		return "", err
	}
	if !strings.HasSuffix(s, "\r") {
		return "", protocolError("expected CRLF")
	}
	return s[:len(s)-1], nil
}

func (rd *Reader) readInlineLine() (string, error) {
	s, err := rd.readUntilNewline(false)
	if err != nil {
		return "", err
	}
	return strings.TrimSuffix(s, "\r"), nil
}

// readUntilNewline reads up to and including the next '\n' and returns the
// bytes before it. requireData reports a mid-value EOF as unexpected when
// the caller has already consumed a type prefix; both forms do that when any
// byte was read.
func (rd *Reader) readUntilNewline(fromTyped bool) (string, error) {
	var b strings.Builder
	for {
		c, err := rd.r.ReadByte()
		if err != nil {
			if err == io.EOF && (b.Len() > 0 || fromTyped) {
				return "", io.ErrUnexpectedEOF
			}
			return "", err
		}
		if c == '\n' {
			return b.String(), nil
		}
		if int64(b.Len()) >= rd.maxBulk {
			return "", protocolError("line too long")
		}
		b.WriteByte(c)
	}
}

func eofInValue(err error) error {
	if err == io.EOF {
		return io.ErrUnexpectedEOF
	}
	return err
}

func splitInline(line string) ([]string, error) {
	var args []string
	i := 0
	for i < len(line) {
		for i < len(line) && isInlineSpace(line[i]) {
			i++
		}
		if i >= len(line) {
			break
		}
		arg, next, err := scanInlineArg(line, i)
		if err != nil {
			return nil, err
		}
		args = append(args, arg)
		i = next
	}
	return args, nil
}

func scanInlineArg(line string, i int) (string, int, error) {
	var b strings.Builder
	var quote byte
	for i < len(line) {
		c := line[i]
		switch quote {
		case 0:
			if isInlineSpace(c) {
				return b.String(), i, nil
			}
			if c == '"' || c == '\'' {
				quote = c
				i++
				continue
			}
			b.WriteByte(c)
			i++
		case '"':
			if c == '\\' {
				if i+1 >= len(line) {
					return "", i, protocolError("unbalanced quotes in inline command")
				}
				nxt := line[i+1]
				if nxt == 'x' && i+3 < len(line) && isHex(line[i+2]) && isHex(line[i+3]) {
					b.WriteByte(hexVal(line[i+2])<<4 | hexVal(line[i+3]))
					i += 4
					continue
				}
				b.WriteByte(unescapeDouble(nxt))
				i += 2
				continue
			}
			if c == '"' {
				if i+1 < len(line) && !isInlineSpace(line[i+1]) {
					return "", i, protocolError("unexpected character after quote")
				}
				return b.String(), i + 1, nil
			}
			b.WriteByte(c)
			i++
		default:
			if c == '\\' && i+1 < len(line) && line[i+1] == '\'' {
				b.WriteByte('\'')
				i += 2
				continue
			}
			if c == '\'' {
				if i+1 < len(line) && !isInlineSpace(line[i+1]) {
					return "", i, protocolError("unexpected character after quote")
				}
				return b.String(), i + 1, nil
			}
			b.WriteByte(c)
			i++
		}
	}
	if quote != 0 {
		return "", i, protocolError("unbalanced quotes in inline command")
	}
	return b.String(), i, nil
}

func isInlineSpace(c byte) bool {
	switch c {
	case ' ', '\t', '\n', '\v', '\f', '\r':
		return true
	default:
		return false
	}
}

func isHex(c byte) bool {
	return (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')
}

func hexVal(c byte) byte {
	switch {
	case c >= '0' && c <= '9':
		return c - '0'
	case c >= 'a' && c <= 'f':
		return c - 'a' + 10
	default:
		return c - 'A' + 10
	}
}

func unescapeDouble(c byte) byte {
	switch c {
	case 'n':
		return '\n'
	case 'r':
		return '\r'
	case 't':
		return '\t'
	case 'a':
		return '\a'
	case 'b':
		return '\b'
	default:
		return c
	}
}

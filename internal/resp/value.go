// Package resp implements the Redis Serialization Protocol, version 2.
package resp

// Type is a RESP2 type prefix.
type Type byte

const (
	TypeSimpleString Type = '+'
	TypeError        Type = '-'
	TypeInteger      Type = ':'
	TypeBulkString   Type = '$'
	TypeArray        Type = '*'
)

// MaxBulkSize is the largest bulk string the reader accepts (512 MiB),
// matching Redis proto-max-bulk-len.
const MaxBulkSize = 512 * 1024 * 1024

// Value is a single RESP2 value. Null is set for null bulk strings ($-1)
// and null arrays (*-1). An empty array has TypeArray, Null false, and an
// empty Array slice.
type Value struct {
	Type  Type
	Str   string
	Int   int64
	Array []Value
	Null  bool
}

// SimpleString returns a RESP simple string.
func SimpleString(s string) Value {
	return Value{Type: TypeSimpleString, Str: s}
}

// Error returns a RESP error. msg is the full error text, including the
// Redis error prefix such as "ERR" or "WRONGTYPE".
func Error(msg string) Value {
	return Value{Type: TypeError, Str: msg}
}

// Integer returns a RESP integer.
func Integer(n int64) Value {
	return Value{Type: TypeInteger, Int: n}
}

// BulkString returns a RESP bulk string. An empty string is $0, not null.
func BulkString(s string) Value {
	return Value{Type: TypeBulkString, Str: s}
}

// NullBulk returns a null bulk string, encoded as $-1.
func NullBulk() Value {
	return Value{Type: TypeBulkString, Null: true}
}

// NullArray returns a null array, encoded as *-1.
func NullArray() Value {
	return Value{Type: TypeArray, Null: true}
}

// Array returns a RESP array. A call with no items is an empty array (*0),
// which is distinct from NullArray.
func Array(items ...Value) Value {
	if items == nil {
		items = []Value{}
	}
	return Value{Type: TypeArray, Array: items}
}

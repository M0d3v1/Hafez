package command

import (
	"strings"

	"github.com/M0d3v1/hafez/internal/resp"
)

// IsWrite reports whether name changes the keyspace when it succeeds.
func IsWrite(name string) bool {
	switch strings.ToUpper(name) {
	case "SET", "MSET", "INCR", "DECR", "INCRBY",
		"DEL", "FLUSHALL",
		"EXPIRE", "PEXPIRE", "PERSIST",
		"LPUSH", "RPUSH", "LPOP", "RPOP",
		"HSET", "HDEL":
		return true
	default:
		return false
	}
}

// Mutated reports whether a successful reply means the keyspace changed.
// Failed conditional writes, such as SET NX on an existing key, return false
// so they are left out of the append-only file.
func Mutated(name string, reply resp.Value) bool {
	if reply.Type == resp.TypeError {
		return false
	}
	switch strings.ToUpper(name) {
	case "SET":
		return reply.Type == resp.TypeSimpleString
	case "MSET", "FLUSHALL", "LPUSH", "RPUSH", "HSET":
		return true
	case "INCR", "DECR", "INCRBY":
		return reply.Type == resp.TypeInteger
	case "DEL", "HDEL":
		return reply.Type == resp.TypeInteger && reply.Int > 0
	case "EXPIRE", "PEXPIRE", "PERSIST":
		return reply.Type == resp.TypeInteger && reply.Int == 1
	case "LPOP", "RPOP":
		return reply.Type == resp.TypeBulkString && !reply.Null
	default:
		return false
	}
}

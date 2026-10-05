// Package store is the in-memory keyspace. Keys are spread across shards so
// callers can use the store from many goroutines.
package store

import "time"

const (
	// ShardCount is the number of independent key maps.
	ShardCount = 256

	// TypeString and TypeNone are the TYPE replies used for string keys.
	TypeString = "string"
	TypeNone   = "none"
)

// ErrWrongType is returned when a command touches a key of another type.
var ErrWrongType = errString("wrong type")

// ErrNotInteger is returned when a value is not a signed 64-bit integer.
var ErrNotInteger = errString("value is not an integer or out of range")

type errString string

func (e errString) Error() string { return string(e) }

// Pair is one key and value for MSET.
type Pair struct {
	Key   string
	Value string
}

// Item is one MGET element. Null is a missing key.
type Item struct {
	Value string
	Null  bool
}

// SetOptions controls SET. A zero TTL with HasTTL false keeps the key forever.
type SetOptions struct {
	NX     bool
	XX     bool
	TTL    time.Duration
	HasTTL bool
}

// Store is the keyspace commands use. Implementations must be safe for
// concurrent use.
type Store interface {
	Set(key, value string, opt SetOptions) bool
	Get(key string) (value string, ok bool, err error)
	Del(keys []string) int
	Exists(keys []string) int
	MSet(pairs []Pair)
	MGet(keys []string) ([]Item, error)
	IncrBy(key string, delta int64) (int64, error)
	Keys(pattern string) []string
	Type(key string) string
	FlushAll()
	DBSize() int
}

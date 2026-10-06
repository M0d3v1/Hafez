// Package store is the in-memory keyspace. Keys are spread across shards so
// callers can use the store from many goroutines.
package store

import "time"

const (
	// ShardCount is the number of independent key maps.
	ShardCount = 256

	// TypeString, TypeList, TypeHash, and TypeNone are TYPE replies.
	TypeString = "string"
	TypeList   = "list"
	TypeHash   = "hash"
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

// Field is one hash field and its value.
type Field struct {
	Name  string
	Value string
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
	// Expire sets a deadline. A non-positive ttl deletes a live key.
	// It returns false when the key is missing.
	Expire(key string, ttl time.Duration) bool
	// Persist clears a deadline. It returns false when the key is missing
	// or already has no deadline.
	Persist(key string) bool
	// TTL reports how long a key still has to live. exists is false when
	// the key is missing. expires is false when the key has no deadline.
	TTL(key string) (remaining time.Duration, exists bool, expires bool)

	LPush(key string, values []string) (int, error)
	RPush(key string, values []string) (int, error)
	LPop(key string) (value string, ok bool, err error)
	RPop(key string) (value string, ok bool, err error)
	LRange(key string, start, stop int64) ([]string, error)
	LLen(key string) (int, error)

	HSet(key string, fields []Field) (added int, err error)
	HGet(key, field string) (value string, ok bool, err error)
	HDel(key string, fields []string) (int, error)
	HGetAll(key string) ([]Field, error)
	HExists(key, field string) (bool, error)
	HLen(key string) (int, error)
}

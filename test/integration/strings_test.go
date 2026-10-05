package integration

import (
	"context"
	"errors"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

func TestStringAndKeyCommands(t *testing.T) {
	rdb := startClient(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := rdb.Set(ctx, "a", "hello", 0).Err(); err != nil {
		t.Fatal(err)
	}
	got, err := rdb.Get(ctx, "a").Result()
	if err != nil || got != "hello" {
		t.Fatalf("GET = %q %v", got, err)
	}
	_, err = rdb.Get(ctx, "missing").Result()
	if !errors.Is(err, redis.Nil) {
		t.Fatalf("missing GET err = %v", err)
	}

	if err := rdb.Do(ctx, "SET", "a", "nope", "NX").Err(); !errors.Is(err, redis.Nil) {
		t.Fatalf("SET NX err = %v", err)
	}
	if err := rdb.Do(ctx, "SET", "b", "there", "EX", "30", "NX").Err(); err != nil {
		t.Fatal(err)
	}
	if err := rdb.Do(ctx, "SET", "missing", "x", "XX").Err(); !errors.Is(err, redis.Nil) {
		t.Fatalf("SET XX err = %v", err)
	}

	if err := rdb.MSet(ctx, "x", "1", "y", "2").Err(); err != nil {
		t.Fatal(err)
	}
	vals, err := rdb.MGet(ctx, "x", "missing", "y").Result()
	if err != nil {
		t.Fatal(err)
	}
	if vals[0] != "1" || vals[1] != nil || vals[2] != "2" {
		t.Fatalf("MGET %#v", vals)
	}

	n, err := rdb.Incr(ctx, "n").Result()
	if err != nil || n != 1 {
		t.Fatalf("INCR = %d %v", n, err)
	}
	n, err = rdb.IncrBy(ctx, "n", 4).Result()
	if err != nil || n != 5 {
		t.Fatalf("INCRBY = %d %v", n, err)
	}
	n, err = rdb.Decr(ctx, "n").Result()
	if err != nil || n != 4 {
		t.Fatalf("DECR = %d %v", n, err)
	}
	if err := rdb.Set(ctx, "word", "hi", 0).Err(); err != nil {
		t.Fatal(err)
	}
	if err := rdb.Incr(ctx, "word").Err(); err == nil || !strings.Contains(err.Error(), "not an integer") {
		t.Fatalf("INCR word err = %v", err)
	}

	exists, err := rdb.Exists(ctx, "a", "nope", "a").Result()
	if err != nil || exists != 2 {
		t.Fatalf("EXISTS = %d %v", exists, err)
	}
	typ, err := rdb.Type(ctx, "a").Result()
	if err != nil || typ != "string" {
		t.Fatalf("TYPE = %q %v", typ, err)
	}
	if typ, err = rdb.Type(ctx, "nope").Result(); err != nil || typ != "none" {
		t.Fatalf("TYPE missing = %q %v", typ, err)
	}

	keys, err := rdb.Keys(ctx, "user:*").Result()
	if err != nil {
		t.Fatal(err)
	}
	if len(keys) != 0 {
		t.Fatalf("KEYS user:* = %v", keys)
	}
	if err := rdb.MSet(ctx, "user:1", "a", "user:2", "b").Err(); err != nil {
		t.Fatal(err)
	}
	keys, err = rdb.Keys(ctx, "user:?").Result()
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(keys)
	if len(keys) != 2 || keys[0] != "user:1" || keys[1] != "user:2" {
		t.Fatalf("KEYS = %v", keys)
	}

	deleted, err := rdb.Del(ctx, "user:1", "missing").Result()
	if err != nil || deleted != 1 {
		t.Fatalf("DEL = %d %v", deleted, err)
	}
	size, err := rdb.DBSize(ctx).Result()
	if err != nil || size == 0 {
		t.Fatalf("DBSIZE = %d %v", size, err)
	}
	if err := rdb.FlushAll(ctx).Err(); err != nil {
		t.Fatal(err)
	}
	size, err = rdb.DBSize(ctx).Result()
	if err != nil || size != 0 {
		t.Fatalf("DBSIZE after flush = %d %v", size, err)
	}
}

package integration

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

func TestListsAndHashes(t *testing.T) {
	rdb := startClient(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	n, err := rdb.LPush(ctx, "l", "a", "b", "c").Result()
	if err != nil || n != 3 {
		t.Fatalf("LPUSH = %d %v", n, err)
	}
	got, err := rdb.LRange(ctx, "l", 0, -1).Result()
	if err != nil || len(got) != 3 || got[0] != "c" || got[1] != "b" || got[2] != "a" {
		t.Fatalf("LRANGE %#v %v", got, err)
	}
	n, err = rdb.RPush(ctx, "l", "d").Result()
	if err != nil || n != 4 {
		t.Fatalf("RPUSH = %d %v", n, err)
	}
	v, err := rdb.LPop(ctx, "l").Result()
	if err != nil || v != "c" {
		t.Fatalf("LPOP = %q %v", v, err)
	}
	v, err = rdb.RPop(ctx, "l").Result()
	if err != nil || v != "d" {
		t.Fatalf("RPOP = %q %v", v, err)
	}
	n, err = rdb.LLen(ctx, "l").Result()
	if err != nil || n != 2 {
		t.Fatalf("LLEN = %d %v", n, err)
	}
	_, err = rdb.LPop(ctx, "missing").Result()
	if !errors.Is(err, redis.Nil) {
		t.Fatalf("LPOP missing err = %v", err)
	}

	if err := rdb.Set(ctx, "s", "v", 0).Err(); err != nil {
		t.Fatal(err)
	}
	err = rdb.LPush(ctx, "s", "a").Err()
	if err == nil || !strings.Contains(err.Error(), "WRONGTYPE") {
		t.Fatalf("LPUSH string err = %v", err)
	}

	added, err := rdb.HSet(ctx, "h", "a", "1", "b", "2").Result()
	if err != nil || added != 2 {
		t.Fatalf("HSET = %d %v", added, err)
	}
	field, err := rdb.HGet(ctx, "h", "a").Result()
	if err != nil || field != "1" {
		t.Fatalf("HGET = %q %v", field, err)
	}
	_, err = rdb.HGet(ctx, "h", "missing").Result()
	if !errors.Is(err, redis.Nil) {
		t.Fatalf("HGET missing err = %v", err)
	}
	ok, err := rdb.HExists(ctx, "h", "b").Result()
	if err != nil || !ok {
		t.Fatalf("HEXISTS %v %v", ok, err)
	}
	n, err = rdb.HLen(ctx, "h").Result()
	if err != nil || n != 2 {
		t.Fatalf("HLEN = %d %v", n, err)
	}
	all, err := rdb.HGetAll(ctx, "h").Result()
	if err != nil || all["a"] != "1" || all["b"] != "2" || len(all) != 2 {
		t.Fatalf("HGETALL %#v %v", all, err)
	}
	removed, err := rdb.HDel(ctx, "h", "b", "missing").Result()
	if err != nil || removed != 1 {
		t.Fatalf("HDEL = %d %v", removed, err)
	}
	err = rdb.HSet(ctx, "s", "a", "1").Err()
	if err == nil || !strings.Contains(err.Error(), "WRONGTYPE") {
		t.Fatalf("HSET string err = %v", err)
	}
	typ, err := rdb.Type(ctx, "h").Result()
	if err != nil || typ != "hash" {
		t.Fatalf("TYPE = %q %v", typ, err)
	}
	typ, err = rdb.Type(ctx, "l").Result()
	if err != nil || typ != "list" {
		t.Fatalf("TYPE list = %q %v", typ, err)
	}
}

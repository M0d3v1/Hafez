package integration

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestExpireCommands(t *testing.T) {
	rdb := startClient(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	n, err := rdb.Do(ctx, "EXPIRE", "missing", "10").Int64()
	if err != nil || n != 0 {
		t.Fatalf("EXPIRE missing = %d %v", n, err)
	}
	n, err = rdb.Do(ctx, "TTL", "missing").Int64()
	if err != nil || n != -2 {
		t.Fatalf("TTL missing = %d %v", n, err)
	}

	if err := rdb.Set(ctx, "k", "v", 0).Err(); err != nil {
		t.Fatal(err)
	}
	n, err = rdb.Do(ctx, "TTL", "k").Int64()
	if err != nil || n != -1 {
		t.Fatalf("TTL persistent = %d %v", n, err)
	}
	n, err = rdb.Do(ctx, "EXPIRE", "k", "30").Int64()
	if err != nil || n != 1 {
		t.Fatalf("EXPIRE = %d %v", n, err)
	}
	n, err = rdb.Do(ctx, "TTL", "k").Int64()
	if err != nil || n < 1 || n > 30 {
		t.Fatalf("TTL = %d %v", n, err)
	}
	n, err = rdb.Do(ctx, "PTTL", "k").Int64()
	if err != nil || n <= 0 || n > 30000 {
		t.Fatalf("PTTL = %d %v", n, err)
	}
	n, err = rdb.Do(ctx, "PERSIST", "k").Int64()
	if err != nil || n != 1 {
		t.Fatalf("PERSIST = %d %v", n, err)
	}
	n, err = rdb.Do(ctx, "TTL", "k").Int64()
	if err != nil || n != -1 {
		t.Fatalf("TTL after PERSIST = %d %v", n, err)
	}
	got, err := rdb.Get(ctx, "k").Result()
	if err != nil || got != "v" {
		t.Fatalf("GET = %q %v", got, err)
	}

	n, err = rdb.Do(ctx, "PEXPIRE", "k", "1500").Int64()
	if err != nil || n != 1 {
		t.Fatalf("PEXPIRE = %d %v", n, err)
	}
	n, err = rdb.Do(ctx, "PTTL", "k").Int64()
	if err != nil || n <= 0 || n > 1500 {
		t.Fatalf("PTTL after PEXPIRE = %d %v", n, err)
	}
	n, err = rdb.Do(ctx, "TTL", "k").Int64()
	if err != nil || n < 0 || n > 1 {
		t.Fatalf("TTL after PEXPIRE = %d %v", n, err)
	}

	err = rdb.Do(ctx, "EXPIRE", "k", "nope").Err()
	if err == nil || !strings.Contains(err.Error(), "not an integer") {
		t.Fatalf("EXPIRE nope err = %v", err)
	}
}

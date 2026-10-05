# Hafez specification

You are a senior Go engineer. Help me build an open-source project called "hafez": a Redis-compatible in-memory key-value server written in Go, from scratch. The goal is a clean, idiomatic, well-tested portfolio project that proves strong Go skills (networking, concurrency, protocols, persistence).

Go module path: github.com/M0d3v1/hafez

## Step 0 (do this first)
Save this entire specification to docs/SPEC.md so we can refer back to it in later sessions. Then show me your implementation plan and wait for my approval.

## Hard constraints
- Go 1.22+. Use ONLY the standard library for the server itself.
- Do NOT use any existing Redis server/protocol libraries. Implement RESP2 myself.
- External packages are allowed ONLY in tests (e.g. github.com/redis/go-redis/v9 for integration tests).
- The server must work with the official `redis-cli` and `redis-benchmark`.
- Idiomatic Go: small packages, clear interfaces, proper error handling, no global mutable state, context-based shutdown.
- Must pass `go test -race ./...` and `go vet ./...` at every phase.

## Project structure
hafez/
├── cmd/hafez/main.go          # entrypoint, flags, signal handling
├── internal/resp/             # RESP2 parser and writer (reader.go, writer.go, value.go)
├── internal/server/           # TCP listener, connection handling, graceful shutdown
├── internal/command/          # command registry + handlers, one file per group (strings.go, keys.go, lists.go, hashes.go, pubsub.go)
├── internal/store/            # sharded concurrent keyspace, expiration
├── internal/persistence/      # AOF writer, replay, rewrite
├── internal/pubsub/           # channel subscriptions
├── test/integration/          # end-to-end tests using go-redis client
├── docs/SPEC.md
├── Makefile, Dockerfile, .golangci.yml
├── .github/workflows/ci.yml
├── LICENSE (MIT)
└── README.md

## Phases (implement ONE phase at a time)
Phase 1 – Foundation
- TCP server on configurable port (default 6379), one goroutine per connection.
- Full RESP2 parser/writer: simple strings, errors, integers, bulk strings, arrays, null bulk. Also support inline commands.
- Commands: PING, ECHO, COMMAND (minimal stub so redis-cli works).
- Unit tests for the RESP parser, including malformed input and partial reads.

Phase 2 – Strings & keys
- Sharded keyspace (e.g. 256 shards, each with sync.RWMutex) behind a Store interface.
- SET (with EX, PX, NX, XX), GET, DEL, EXISTS, MSET, MGET, INCR, DECR, INCRBY, KEYS (glob pattern), TYPE, FLUSHALL, DBSIZE.
- Correct Redis error messages (e.g. WRONGTYPE, ERR value is not an integer).

Phase 3 – Expiration
- EXPIRE, PEXPIRE, TTL, PTTL, PERSIST.
- Lazy expiration on access + active expiration (background goroutine sampling random keys, like real Redis).
- Tests using a fake/injectable clock, not time.Sleep.

Phase 4 – Lists & hashes
- LPUSH, RPUSH, LPOP, RPOP, LRANGE, LLEN.
- HSET, HGET, HDEL, HGETALL, HEXISTS, HLEN.

Phase 5 – Persistence (AOF)
- Append-only file logging every write command in RESP format.
- fsync policies via flag: always / everysec / no.
- Replay AOF on startup; tolerate a truncated last entry.
- BGREWRITEAOF: compact the AOF from current state without blocking clients.

Phase 6 – Pub/Sub
- SUBSCRIBE, UNSUBSCRIBE, PUBLISH, PSUBSCRIBE (glob patterns).
- Subscribed connections enter pub/sub mode correctly.

Phase 7 – Production polish
- Graceful shutdown on SIGINT/SIGTERM (stop accepting, drain connections, flush AOF).
- INFO command (uptime, connected clients, keys, memory usage via runtime).
- Config via flags: --port, --aof, --appendfsync, --maxclients.
- Structured logging with log/slog.
- Benchmarks: Go benchmarks for store and RESP; document redis-benchmark results.

## Tooling & docs (after Phase 7)
- Makefile: build, test, race, lint, bench, run.
- Multi-stage Dockerfile (small final image).
- GitHub Actions: go vet, golangci-lint, go test -race, build on push and PR.
- README.md with:
  - Title and one-line description.
  - A short "About the name" note: "Hafez (حافظ) means 'the keeper' in Persian, and shares its root with the word for memory. It's also the name of a Persian poet whose words have stayed in people's memory for over 600 years."
  - Features, supported commands table, architecture diagram (Mermaid).
  - Quick start: go install, go run, Docker, and a redis-cli example session.
  - Design decisions: why sharding, how expiration works, how AOF works.
  - Benchmark table, roadmap, MIT license.

## How to work with me
- Implement only the current phase, then STOP and wait for me.
- After each phase: list the files changed, explain the key design decisions in simple terms, tell me exactly how to test it manually with redis-cli, and suggest 1–3 small git commit messages (conventional commits style).
- Never skip tests. Never leave TODOs in delivered code.
- If something is ambiguous, ask instead of guessing.
- In new sessions, re-read docs/SPEC.md before continuing.

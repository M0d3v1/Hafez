.PHONY: build test race vet bench run clean

build:
	go build -trimpath -o bin/hafez ./cmd/hafez

test:
	go test ./...

race:
	go test -race ./...

vet:
	go vet ./...

bench:
	go test -bench=. -benchmem -count=1 ./internal/resp ./internal/store

run:
	go run ./cmd/hafez --port 6379

clean:
	rm -rf bin

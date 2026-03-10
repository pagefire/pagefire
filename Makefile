.PHONY: build dev test lint clean

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS := -ldflags "-X main.version=$(VERSION)"

build:
	go build $(LDFLAGS) -o bin/pagefire ./cmd/pagefire

dev:
	go run ./cmd/pagefire serve

test:
	go test ./...

lint:
	golangci-lint run ./...

clean:
	rm -rf bin/ tmp/

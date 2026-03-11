.PHONY: build dev test lint clean frontend

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS := -ldflags "-X main.version=$(VERSION)"

frontend:
	cd web && npm ci && npm run build

build: frontend
	go build $(LDFLAGS) -o bin/pagefire ./cmd/pagefire

dev:
	go run ./cmd/pagefire serve

dev-frontend:
	cd web && npm run dev

test:
	go test ./...

lint:
	golangci-lint run ./...

clean:
	rm -rf bin/ tmp/ web/dist/

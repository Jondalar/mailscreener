BINARY := screenerd
PKG := ./cmd/screenerd
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X main.version=$(VERSION)

.PHONY: build run test lint tidy docker clean

build:
	go build -ldflags '$(LDFLAGS)' -o bin/$(BINARY) $(PKG)

run:
	go run $(PKG)

# Local dev/test run on the Mac: loads .env, uses ./state for the DB + legacy
# seed, human-readable logs. Production runs on the NAS via docker-compose.
dev: build
	@set -a; . ./.env; set +a; STATE_DIR=$$PWD/state LOG_FORMAT=text ./bin/$(BINARY)

# Seed the local dev DB from the legacy lists in ./state/legacy-seed.
dev-migrate: build
	@set -a; . ./.env; set +a; STATE_DIR=$$PWD/state ./bin/$(BINARY) migrate ./state/legacy-seed

test:
	go test ./...

lint:
	golangci-lint run

tidy:
	go mod tidy

docker:
	docker build -t mailscreener/screenerd:$(VERSION) .

clean:
	rm -rf bin dist

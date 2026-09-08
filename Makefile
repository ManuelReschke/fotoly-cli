.PHONY: build test
VERSION ?= dev
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)

build:
	mkdir -p bin
	go build -ldflags "-X main.version=$(VERSION) -X main.commit=$(COMMIT)" -o bin/fotoly ./cmd/fotoly
	go build -ldflags "-X main.version=$(VERSION) -X main.commit=$(COMMIT)" -o bin/pixelfox ./cmd/pixelfox

test:
	go test ./...

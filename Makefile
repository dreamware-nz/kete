.PHONY: build test docs clean

VERSION ?= dev

build:
	go build -ldflags "-X main.version=$(VERSION)" -o bin/kete ./cmd/kete

test:
	go test ./...

docs:
	go run ./cmd/ketedoc docs/reference/cli.md

clean:
	rm -rf bin

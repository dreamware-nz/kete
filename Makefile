.PHONY: build test docs clean release

VERSION ?= 0.1.0

build:
	go build -ldflags "-X main.version=$(VERSION)" -o bin/kete ./cmd/kete

test:
	go test ./...

docs:
	go run ./cmd/ketedoc docs/reference/cli.md

clean:
	rm -rf bin dist

# release builds darwin/{amd64,arm64} + linux/{amd64,arm64} static
# binaries under dist/ . CGO_ENABLED=0 stays off because we're on
# pure-Go SQLite (modernc.org/sqlite); ADR 0002.
release: clean
	mkdir -p dist
	GOOS=darwin  GOARCH=amd64 CGO_ENABLED=0 go build -ldflags "-X main.version=$(VERSION)" -o dist/kete-darwin-amd64  ./cmd/kete
	GOOS=darwin  GOARCH=arm64 CGO_ENABLED=0 go build -ldflags "-X main.version=$(VERSION)" -o dist/kete-darwin-arm64  ./cmd/kete
	GOOS=linux   GOARCH=amd64 CGO_ENABLED=0 go build -ldflags "-X main.version=$(VERSION)" -o dist/kete-linux-amd64   ./cmd/kete
	GOOS=linux   GOARCH=arm64 CGO_ENABLED=0 go build -ldflags "-X main.version=$(VERSION)" -o dist/kete-linux-arm64   ./cmd/kete
	@echo "Built:"
	@ls -lh dist/

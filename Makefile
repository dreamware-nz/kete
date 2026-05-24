.PHONY: build test docs clean release install uninstall

VERSION ?= 0.1.0
PREFIX  ?= $(HOME)/.local

build:
	go build -ldflags "-X main.version=$(VERSION)" -o bin/kete ./cmd/kete

test:
	go test ./...

docs:
	go run ./cmd/ketedoc docs/reference/cli.md

clean:
	rm -rf bin dist

# install builds and copies the binary to $PREFIX/bin (default
# ~/.local/bin). Honours $PREFIX so distros and homebrew can
# `make install PREFIX=/usr/local`.
install: build
	@mkdir -p $(PREFIX)/bin
	install -m 0755 bin/kete $(PREFIX)/bin/kete
	@echo "Installed $(PREFIX)/bin/kete"
	@case ":$$PATH:" in \
		*":$(PREFIX)/bin:"*) ;; \
		*) echo "WARNING: $(PREFIX)/bin is not on your PATH"; \
		   echo "  add this to your shell rc:"; \
		   echo "    export PATH=\"$(PREFIX)/bin:\$$PATH\"" ;; \
	esac

uninstall:
	rm -f $(PREFIX)/bin/kete
	@echo "Removed $(PREFIX)/bin/kete (your ~/.kete/ data is untouched; use 'kete purge' to nuke that)"

# release builds darwin/{amd64,arm64} + linux/{amd64,arm64} static
# binaries under dist/ . CGO_ENABLED=0 stays off because we're on
# pure-Go SQLite (modernc.org/sqlite); ADR 0002.
release: clean
	mkdir -p dist
	GOOS=darwin  GOARCH=amd64 CGO_ENABLED=0 go build -ldflags "-X main.version=$(VERSION)" -o dist/kete-darwin-amd64  ./cmd/kete
	GOOS=darwin  GOARCH=arm64 CGO_ENABLED=0 go build -ldflags "-X main.version=$(VERSION)" -o dist/kete-darwin-arm64  ./cmd/kete
	GOOS=linux   GOARCH=amd64 CGO_ENABLED=0 go build -ldflags "-X main.version=$(VERSION)" -o dist/kete-linux-amd64   ./cmd/kete
	GOOS=linux   GOARCH=arm64 CGO_ENABLED=0 go build -ldflags "-X main.version=$(VERSION)" -o dist/kete-linux-arm64   ./cmd/kete
	@cd dist && shasum -a 256 kete-* > SHA256SUMS
	@echo "Built:"
	@ls -lh dist/

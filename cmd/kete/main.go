// Command kete is the local memory and reasoning layer for Crush sessions.
package main

import (
	"github.com/dreamware-nz/kete/internal/cli"
)

// version is plumbed through to internal/cli at startup.
//
//	go build -ldflags "-X main.version=0.1.0" ./cmd/kete
var version = "dev"

func main() {
	cli.Version = version
	cli.Main()
}

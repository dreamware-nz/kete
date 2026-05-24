// Command ketedoc regenerates docs/reference/cli.md from the live cobra
// command tree. Invoked by `make docs`.
package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/dreamware-nz/kete/internal/cli"
	"github.com/spf13/cobra"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: ketedoc <output-file>")
		os.Exit(1)
	}
	root := cli.NewRoot()
	var buf bytes.Buffer
	buf.WriteString("# CLI reference\n\n")
	buf.WriteString("> Generated from `kete --help`. Run `make docs` to regenerate.\n\n")
	walk(&buf, root, 2)
	if err := os.WriteFile(os.Args[1], buf.Bytes(), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func walk(w io.Writer, c *cobra.Command, level int) {
	if c.Hidden {
		return
	}
	hashes := strings.Repeat("#", level)
	fmt.Fprintf(w, "%s `%s`\n\n", hashes, c.CommandPath())
	if c.Short != "" {
		fmt.Fprintf(w, "%s\n\n", c.Short)
	}
	if c.Long != "" {
		fmt.Fprintf(w, "%s\n\n", c.Long)
	}
	fmt.Fprintf(w, "```\n%s\n```\n\n", c.UseLine())
	if c.HasAvailableLocalFlags() {
		fmt.Fprintln(w, "Flags:")
		fmt.Fprintln(w, "")
		fmt.Fprintln(w, "```")
		fmt.Fprintln(w, c.LocalFlags().FlagUsages())
		fmt.Fprintln(w, "```")
		fmt.Fprintln(w, "")
	}
	for _, sub := range c.Commands() {
		walk(w, sub, level+1)
	}
}

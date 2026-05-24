package inject

import (
	"fmt"
	"strings"

	"github.com/dreamware-nz/kete/internal/store"
)

// Preview renders one prior task as a short text block suitable for
// splicing into a request body's user content. The "id" attribute is
// the 8-char ShortID — the same id the MCP server resolves via
// kete_expand. Same shape proxy and MCP both use, so no second
// implementation drifts.
func Preview(t *store.Task) string {
	var b strings.Builder
	fmt.Fprintf(&b, "<kete:memory id=%q created=%q>\n",
		ShortID(t.ID), t.CreatedAt.UTC().Format("2006-01-02"))
	if t.Goal != "" {
		fmt.Fprintf(&b, "  <goal>%s</goal>\n", escapeXML(t.Goal))
	}
	for _, d := range t.Decisions {
		fmt.Fprintf(&b, "  <decision choice=%q rationale=%q/>\n",
			d.Choice, d.Rationale)
	}
	if len(t.FilesTouched) > 0 {
		fmt.Fprintf(&b, "  <files>%s</files>\n", strings.Join(t.FilesTouched, ", "))
	}
	b.WriteString("</kete:memory>\n")
	return b.String()
}

func escapeXML(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch r {
		case '<':
			b.WriteString("&lt;")
		case '>':
			b.WriteString("&gt;")
		case '&':
			b.WriteString("&amp;")
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

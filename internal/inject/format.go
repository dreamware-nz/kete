package inject

import (
	"fmt"
	"strings"

	"github.com/dreamware-nz/kete/internal/store"
)

// Preview renders one prior task as a short text block suitable for
// splicing into a request body's user content. Stable shape so the
// model sees the same structure across calls.
func Preview(t *store.Task) string {
	var b strings.Builder
	fmt.Fprintf(&b, "<kete:memory id=%q created=%q>\n",
		t.ID, t.CreatedAt.UTC().Format("2006-01-02"))
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

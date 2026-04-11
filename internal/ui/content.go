package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/glamour"
	"github.com/peterqlin/lazydict/internal/api"
)

func RenderEntry(entry *api.Entry, width int) string {
	if entry == nil {
		return ""
	}
	md := buildMarkdown(entry)
	r, err := glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithWordWrap(width),
	)
	if err != nil {
		return md
	}
	out, err := r.Render(md)
	if err != nil {
		return md
	}
	return out
}

func RenderError(msg string) string {
	md := fmt.Sprintf("## Error\n\n%s\n", msg)
	r, _ := glamour.NewTermRenderer(glamour.WithAutoStyle())
	if r == nil {
		return msg
	}
	out, _ := r.Render(md)
	return out
}

func RenderWelcome() string {
	md := "# lazydict\n\nPress **i** to search, or type a word and press **Enter**.\n"
	r, _ := glamour.NewTermRenderer(glamour.WithAutoStyle())
	if r == nil {
		return md
	}
	out, _ := r.Render(md)
	return out
}

func buildMarkdown(e *api.Entry) string {
	var b strings.Builder

	fmt.Fprintf(&b, "# %s\n", e.Word)
	if e.Pronunciation != "" {
		fmt.Fprintf(&b, "`%s`\n", e.Pronunciation)
	}
	b.WriteString("\n---\n\n")

	for _, group := range e.DefinitionGroups {
		if group.POS != "" {
			fmt.Fprintf(&b, "## %s\n\n", group.POS)
		}
		for _, d := range group.Defs {
			fmt.Fprintf(&b, "%s\n\n", d)
		}
	}

	if len(e.Synonyms) > 0 {
		b.WriteString("## Synonyms\n\n")
		fmt.Fprintf(&b, "%s\n\n", strings.Join(e.Synonyms, ", "))
	}

	if len(e.Antonyms) > 0 {
		b.WriteString("## Antonyms\n\n")
		fmt.Fprintf(&b, "%s\n\n", strings.Join(e.Antonyms, ", "))
	}

	if len(e.Examples) > 0 {
		b.WriteString("## Examples\n\n")
		for _, ex := range e.Examples {
			fmt.Fprintf(&b, "> %s\n\n", ex)
		}
	}

	if len(e.Forms) > 0 {
		b.WriteString("## Forms\n\n")
		fmt.Fprintf(&b, "%s\n\n", strings.Join(e.Forms, ", "))
	}

	if e.Etymology != "" {
		b.WriteString("## Etymology\n\n")
		fmt.Fprintf(&b, "%s\n\n", e.Etymology)
	}

	return b.String()
}

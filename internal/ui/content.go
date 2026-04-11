package ui

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/reflow/wordwrap"

	"github.com/peterqlin/lazydict/internal/api"
)

// posAbbrev returns a short display label for a part-of-speech string.
func posAbbrev(pos string) string {
	switch pos {
	case "adjective":
		return "adj"
	case "adverb":
		return "adv"
	case "noun":
		return "noun"
	case "verb":
		return "verb"
	default:
		if len(pos) <= 4 {
			return pos
		}
		return pos[:4]
	}
}

// RenderEntry renders a dictionary entry in the dense-reference style.
func RenderEntry(entry *api.Entry, width int) string {
	if entry == nil {
		return ""
	}
	if width < 10 {
		width = 10
	}
	var b strings.Builder

	// Header line: word  [pos-abbrev if single-POS]  pronunciation
	wordStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#e6edf3"))
	headerParts := []string{wordStyle.Render(entry.Word)}
	if len(entry.DefinitionGroups) == 1 && entry.DefinitionGroups[0].POS != "" {
		abbr := lipgloss.NewStyle().Foreground(ColorMuted).Render(posAbbrev(entry.DefinitionGroups[0].POS))
		headerParts = append(headerParts, abbr)
	}
	if entry.Pronunciation != "" {
		pron := lipgloss.NewStyle().Foreground(ColorAccent).Render(entry.Pronunciation)
		headerParts = append(headerParts, pron)
	}
	fmt.Fprintln(&b, strings.Join(headerParts, "  "))

	// Forms line — only when non-empty
	if len(entry.Forms) > 0 {
		fmt.Fprintln(&b, lipgloss.NewStyle().Foreground(ColorMuted).Render(strings.Join(entry.Forms, " · ")))
	}

	// Divider
	fmt.Fprintln(&b, lipgloss.NewStyle().Foreground(ColorDim).Render(strings.Repeat("─", width)))

	// Definitions grouped by POS
	numStyle := lipgloss.NewStyle().Bold(true).Foreground(ColorBlue)
	posStyle := lipgloss.NewStyle().Foreground(ColorGold)
	multiPOS := len(entry.DefinitionGroups) >= 2
	for i, group := range entry.DefinitionGroups {
		if i > 0 {
			fmt.Fprintln(&b)
		}
		if multiPOS {
			fmt.Fprintln(&b, posStyle.Render(group.POS))
		}
		for j, def := range group.Defs {
			b.WriteString(formatDef(numStyle, j+1, def, width))
		}
	}
	fmt.Fprintln(&b)

	// Footer rows: syn / ant / ex / etym
	labelStyle := lipgloss.NewStyle().Foreground(ColorDim)
	synStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#3fb950"))
	antStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#f85149"))
	exStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#8b949e"))
	mutedStyle := lipgloss.NewStyle().Foreground(ColorMuted)

	if len(entry.Synonyms) > 0 {
		b.WriteString(formatFooterRow(labelStyle, "syn", synStyle, strings.Join(entry.Synonyms, " · "), width))
	}
	if len(entry.Antonyms) > 0 {
		b.WriteString(formatFooterRow(labelStyle, "ant", antStyle, strings.Join(entry.Antonyms, " · "), width))
	}
	for _, ex := range entry.Examples {
		b.WriteString(formatFooterRow(labelStyle, "ex", exStyle, ex, width))
	}
	if entry.Etymology != "" {
		b.WriteString(formatFooterRow(labelStyle, "etym", mutedStyle, entry.Etymology, width))
	}

	return b.String()
}

// RenderError renders an error message in the content pane.
func RenderError(msg string) string {
	return lipgloss.NewStyle().Foreground(lipgloss.Color("#f85149")).Render(msg) + "\n"
}

// RenderWelcome renders the welcome screen shown before any lookup.
func RenderWelcome() string {
	title := lipgloss.NewStyle().Bold(true).Foreground(ColorBlue).Render("lazydict")
	hint := lipgloss.NewStyle().Foreground(ColorMuted).Render("press i to search")
	return "\n" + title + "\n\n" + hint + "\n"
}

// formatDef formats one definition line with a hanging-indent number prefix.
// Output example (width=40):
//
//	1 lasting a very short time
//	2 living or lasting only one day (of
//	  certain insects and plants)
func formatDef(numStyle lipgloss.Style, n int, text string, width int) string {
	textWidth := width - 2 // "N " prefix is 2 chars
	if textWidth < 1 {
		textWidth = 1
	}
	lines := strings.Split(strings.TrimRight(wordwrap.String(text, textWidth), "\n"), "\n")
	var b strings.Builder
	for i, line := range lines {
		if i == 0 {
			fmt.Fprintf(&b, "%s %s\n", numStyle.Render(strconv.Itoa(n)), line)
		} else {
			fmt.Fprintf(&b, "  %s\n", line)
		}
	}
	return b.String()
}

// formatFooterRow formats a labeled footer row with word-wrap.
// Label is left-padded to 4 chars; values start at column 5.
// Output example:
//
//	syn  transient · fleeting · momentary
//	     brief · short-lived
func formatFooterRow(labelStyle lipgloss.Style, label string, valStyle lipgloss.Style, value string, width int) string {
	textWidth := width - 5 // "lbl  " prefix is 5 chars
	if textWidth < 1 {
		textWidth = 1
	}
	lines := strings.Split(strings.TrimRight(wordwrap.String(value, textWidth), "\n"), "\n")
	var b strings.Builder
	for i, line := range lines {
		if i == 0 {
			fmt.Fprintf(&b, "%s %s\n", labelStyle.Render(fmt.Sprintf("%-4s", label)), valStyle.Render(line))
		} else {
			fmt.Fprintf(&b, "     %s\n", valStyle.Render(line))
		}
	}
	return b.String()
}

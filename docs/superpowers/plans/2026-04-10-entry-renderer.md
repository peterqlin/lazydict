# Entry Renderer Redesign — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace glamour Markdown rendering with a purpose-built lipgloss renderer using the dense-reference visual style, and restructure `Entry` to group definitions by part of speech.

**Architecture:** Add `PosGroup` type to the API layer and replace `Entry.Definitions []string` + `Entry.FunctionalLabel` with `Entry.DefinitionGroups []PosGroup`. Rewrite `content.go` to render directly with lipgloss styles (no external renderer). Drop glamour as a dependency.

**Tech Stack:** `github.com/charmbracelet/lipgloss`, `github.com/muesli/reflow/wordwrap` (already indirect dep — promoted by `go mod tidy`).

---

### Task 1: Restructure Entry in mw.go and update mw_test.go

**Files:**
- Modify: `internal/api/mw.go`
- Modify: `internal/api/mw_test.go`
- Modify: `internal/ui/content.go` (minimal patch to keep compilation after struct change)

- [ ] **Step 1: Update mw_test.go to use the new struct shape (test will fail to compile)**

Replace lines 49–56 in `internal/api/mw_test.go` (the three assertions after `entry.Etymology`):

```go
// Remove:
	if entry.FunctionalLabel != "noun" {
		t.Errorf("FL = %q, want %q", entry.FunctionalLabel, "noun")
	}
	if len(entry.Definitions) == 0 {
		t.Error("expected at least one definition")
	}
	if entry.Definitions[0] != "1. a thing constituting evidence" {
		t.Errorf("def = %q", entry.Definitions[0])
	}

// Replace with:
	if len(entry.DefinitionGroups) == 0 {
		t.Fatal("expected at least one definition group")
	}
	if entry.DefinitionGroups[0].POS != "noun" {
		t.Errorf("POS = %q, want noun", entry.DefinitionGroups[0].POS)
	}
	if len(entry.DefinitionGroups[0].Defs) == 0 {
		t.Error("expected at least one definition in group")
	}
	if entry.DefinitionGroups[0].Defs[0] != "a thing constituting evidence" {
		t.Errorf("def = %q", entry.DefinitionGroups[0].Defs[0])
	}
```

Also add the multi-POS fixture and test at the bottom of `mw_test.go`, before the final `}`:

```go
const dictMultiPOSFixture = `[
  {
    "meta": {"id": "record:1", "stems": ["record"]},
    "hwi": {"hw": "record", "prs": [{"mw": "ˈre-kərd"}]},
    "fl": "noun",
    "def": [{"sseq": [[["sense", {"sn": "1", "dt": [["text", "a piece of evidence"]]}]]]}]
  },
  {
    "meta": {"id": "record:2", "stems": ["record"]},
    "hwi": {"hw": "record"},
    "fl": "verb",
    "def": [{"sseq": [[["sense", {"sn": "1", "dt": [["text", "to set down in writing"]]}]]]}]
  }
]`

func TestFetch_MultiPOS(t *testing.T) {
	dictSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(dictMultiPOSFixture))
	}))
	defer dictSrv.Close()

	thesSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`[]`))
	}))
	defer thesSrv.Close()

	client := api.NewClient("k", "k",
		api.WithDictBaseURL(dictSrv.URL+"/"),
		api.WithThesBaseURL(thesSrv.URL+"/"),
	)

	entry, err := client.Fetch("record")
	if err != nil {
		t.Fatalf("Fetch error: %v", err)
	}
	if len(entry.DefinitionGroups) != 2 {
		t.Fatalf("expected 2 POS groups, got %d", len(entry.DefinitionGroups))
	}
	if entry.DefinitionGroups[0].POS != "noun" {
		t.Errorf("group[0].POS = %q, want noun", entry.DefinitionGroups[0].POS)
	}
	if entry.DefinitionGroups[1].POS != "verb" {
		t.Errorf("group[1].POS = %q, want verb", entry.DefinitionGroups[1].POS)
	}
}
```

- [ ] **Step 2: Verify the test fails to compile**

```bash
go test ./internal/api/...
```

Expected: compile error — `entry.FunctionalLabel` undefined, `entry.DefinitionGroups` undefined.

- [ ] **Step 3: Update mw.go — add PosGroup, restructure Entry, fix buildEntry and extractDefs**

Replace the `Entry` struct (lines 20–30) in `internal/api/mw.go`:

```go
// PosGroup holds definitions for a single part of speech.
type PosGroup struct {
	POS  string
	Defs []string
}

// Entry is the normalised result for a single lookup.
type Entry struct {
	Word             string
	Pronunciation    string
	DefinitionGroups []PosGroup
	Synonyms         []string
	Antonyms         []string
	Examples         []string
	Forms            []string
	Etymology        string
}
```

In `buildEntry`, remove the FunctionalLabel collection block (lines 221–229):

```go
// DELETE these lines entirely:
	seen := map[string]bool{first.FL: true}
	labels := []string{first.FL}
	for _, d := range dict[1:] {
		if !seen[d.FL] {
			seen[d.FL] = true
			labels = append(labels, d.FL)
		}
	}
	e.FunctionalLabel = strings.Join(labels, " / ")
```

Replace the definition-collection loop (lines 231–237) with PosGroup-based grouping:

```go
// Replace:
	for _, d := range dict {
		for _, def := range d.Def {
			defs, exs := extractDefs(def.SSeq)
			e.Definitions = append(e.Definitions, defs...)
			e.Examples = append(e.Examples, exs...)
		}
	}

// With:
	for _, d := range dict {
		group := PosGroup{POS: d.FL}
		for _, def := range d.Def {
			defs, exs := extractDefs(def.SSeq)
			group.Defs = append(group.Defs, defs...)
			e.Examples = append(e.Examples, exs...)
		}
		if len(group.Defs) > 0 {
			e.DefinitionGroups = append(e.DefinitionGroups, group)
		}
	}
```

In `extractDefs`, simplify the `"text"` case (lines 296–309) to drop the embedded sense number:

```go
// Replace:
				if sd.SN != "" {
					defs = append(defs, sd.SN+". "+text)
				} else {
					defs = append(defs, text)
				}

// With:
				defs = append(defs, text)
```

- [ ] **Step 4: Patch content.go to fix compilation (FunctionalLabel and Definitions are gone)**

Replace lines 54–69 in `internal/ui/content.go` (the FunctionalLabel block and the Definitions block):

```go
// Remove:
	if e.FunctionalLabel != "" || e.Pronunciation != "" {
		parts := []string{}
		if e.FunctionalLabel != "" {
			parts = append(parts, "*"+e.FunctionalLabel+"*")
		}
		if e.Pronunciation != "" {
			parts = append(parts, "`"+e.Pronunciation+"`")
		}
		fmt.Fprintf(&b, "%s\n", strings.Join(parts, " · "))
	}
	b.WriteString("\n---\n\n")

	if len(e.Definitions) > 0 {
		b.WriteString("## Definitions\n\n")
		for _, d := range e.Definitions {
			fmt.Fprintf(&b, "%s\n\n", d)
		}
	}

// Replace with:
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
```

- [ ] **Step 5: Verify tests pass and build succeeds**

```bash
go test ./internal/api/... -v
go build ./...
```

Expected: all api tests pass (TestFetch_HappyPath, TestFetch_NotFound, TestFetch_MultiPOS, TestCleanMarkup); build succeeds.

- [ ] **Step 6: Commit**

```bash
git add internal/api/mw.go internal/api/mw_test.go internal/ui/content.go
git commit -m "feat: restructure Entry with PosGroup, group definitions by POS"
```

---

### Task 2: Rewrite content.go with lipgloss dense-reference renderer

**Files:**
- Modify: `internal/ui/content.go` (full rewrite)

No automated tests — rendering is verified visually by running the binary.

- [ ] **Step 1: Replace content.go entirely**

Write the following as the complete contents of `internal/ui/content.go`:

```go
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
```

- [ ] **Step 2: Verify the build compiles cleanly**

```bash
go build ./...
```

Expected: no errors.

- [ ] **Step 3: Run all tests**

```bash
go test ./...
```

Expected:
```
ok  	github.com/peterqlin/lazydict/config
ok  	github.com/peterqlin/lazydict/internal/api
ok  	github.com/peterqlin/lazydict/internal/app
ok  	github.com/peterqlin/lazydict/internal/store
```

- [ ] **Step 4: Commit**

```bash
git add internal/ui/content.go
git commit -m "feat: replace glamour renderer with lipgloss dense-reference style"
```

---

### Task 3: Remove glamour and tidy go.mod

**Files:**
- Modify: `go.mod`, `go.sum` (via `go mod tidy`)

- [ ] **Step 1: Run go mod tidy**

```bash
go mod tidy
```

Expected: `go.mod` no longer lists `github.com/charmbracelet/glamour`. Several indirect deps that glamour brought in (alecthomas/chroma, microcosm-cc/bluemonday, gorilla/css, yuin/goldmark, yuin/goldmark-emoji, aymerick/douceur) are also removed. `github.com/muesli/reflow` loses its `// indirect` comment.

- [ ] **Step 2: Verify build and tests still pass**

```bash
go build ./... && go test ./...
```

Expected: same pass/fail as after Task 2.

- [ ] **Step 3: Commit**

```bash
git add go.mod go.sum
git commit -m "chore: remove glamour, promote reflow to direct dep"
```

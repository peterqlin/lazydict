# Entry Renderer Redesign — Design Spec
_2026-04-10_

## Goal

Replace the glamour-based Markdown renderer for dictionary entries with a purpose-built lipgloss renderer using a "Dense Reference" visual style. Remove glamour as a dependency entirely.

---

## Visual Design

### Single-POS word (`ephemeral`)

```
ephemeral  adj  i-ˈfe-mə-rəl
──────────────────────────────────────────
1 lasting a very short time
2 living or lasting only one day (of
  certain insects and plants)

syn  transient · fleeting · momentary
     brief · short-lived
ant  permanent · enduring
ex   fame is as ephemeral as fashion
etym Greek ephemeros (epi- + hemera day)
```

### Multi-POS word (`record`)

```
record  ˈre-kərd / ri-ˈkȯrd
recorded · recording · records
──────────────────────────────────────────
noun
1 a thing constituting a piece of evidence
  about past events
2 the best performance or most remarkable
  event of its kind

verb
1 set down in writing for future reference
2 convert (sound/TV) to audio or video for
  later playback

syn  register · document · chronicle
ant  erase · delete
ex   she kept a careful record of expenses
etym Anglo-French recorder, to recall
```

---

## Rendering Rules

### Header (always present)

Line 1: `{word}  {pos-abbrev}  {pronunciation}`
- **word** — bold, bright white (`#e6edf3`)
- **pos-abbrev** — only rendered for single-POS words; muted gray (`#6e7681`); abbreviated form of the POS string (e.g. `adj`, `noun`, `verb`, `adv`)
- **pronunciation** — accent blue (`#79c0ff`); omitted if empty

Line 2 (forms): `{form1} · {form2} · ...`
- Only rendered when `Entry.Forms` is non-empty
- Color: muted (`#6e7681`)
- Multi-POS words always show the forms line here; single-POS words only show it when forms are present

### Divider

`────────` repeated to fill the pane width, color dim (`#444d56`).

### POS blocks

Rendered only when `len(Entry.DefinitionGroups) >= 2`.

Each block:
```
{pos}
{n} {definition text}
{n} {definition text}
```

- POS label: gold (`#d2a679`), no extra formatting
- Blank line between POS blocks
- Numbering restarts at 1 within each block

When only one POS group exists, skip the POS label — definitions render directly after the divider.

### Definition lines

```
{n} {text}
```

- `{n}` — blue (`#58a6ff`), bold; right-aligned in a 1-char column
- One space between number and text
- Wrapped continuation lines align to the text column (2-char indent: `  `)

### Footer rows

Each row: `{label}  {value}`

| Label | Color | Value format |
|-------|-------|--------------|
| `syn` | dim (`#444d56`) | synonyms joined with ` · `, green (`#3fb950`); continuation lines indented 5 chars |
| `ant` | dim | antonyms joined with ` · `, red (`#f85149`); same wrap rule |
| `ex`  | dim | one example per `ex` row, muted italic (`#8b949e`); multiple examples each get their own `ex` row |
| `etym`| dim | etymology string, muted gray (`#6e7681`) |

Labels are left-padded to 4 characters, followed by 2 spaces, giving a consistent value start column of 6. Omit any row whose data is empty.

Order: syn → ant → ex (one row per example) → etym.

### Error view

```
  error  {message}
```

Centered vertically in the viewport. Message in muted red (`#f85149`).

### Welcome view

```
  lazydict

  press i to search
```

Centered. "lazydict" in accent blue, instruction in muted gray.

---

## Data Model Changes

### New type in `internal/api/mw.go`

```go
// PosGroup holds definitions for a single part of speech.
type PosGroup struct {
    POS  string   // e.g. "noun", "verb", "adjective"
    Defs []string // cleaned definition strings
}
```

### Updated `Entry` struct

```go
type Entry struct {
    Word             string
    Pronunciation    string
    DefinitionGroups []PosGroup // replaces Definitions []string
    Synonyms         []string
    Antonyms         []string
    Examples         []string
    Forms            []string
    Etymology        string
}
```

`FunctionalLabel string` is removed. POS information lives entirely in `DefinitionGroups`.

### Updated `buildEntry` in `mw.go`

For each `rawDictEntry d` in `dict`:
1. Create a `PosGroup{POS: d.FL}`
2. Call `extractDefs(def.SSeq)` for each def block; append defs to the group
3. Collect examples globally into `e.Examples`
4. Append the group to `e.DefinitionGroups` only if it has at least one definition

### POS abbreviation helper

```go
func posAbbrev(pos string) string
```

Maps full POS strings to display abbreviations:
- `"adjective"` → `"adj"`
- `"adverb"` → `"adv"`
- `"noun"` → `"noun"`
- `"verb"` → `"verb"`
- anything else → first 4 chars of `pos`, or `pos` if shorter

---

## Implementation

### `internal/api/mw.go`

- Add `PosGroup` type above `Entry`
- Remove `FunctionalLabel` from `Entry`; add `DefinitionGroups []PosGroup`
- Rewrite the definition-collection loop in `buildEntry` to build `[]PosGroup`

### `internal/ui/content.go`

Remove glamour import. Replace with lipgloss. New public API:

```go
func RenderEntry(entry *Entry, width int) string
func RenderError(msg string) string
func RenderWelcome() string
```

All three render directly with lipgloss — no glamour, no Markdown.

`buildMarkdown` is deleted.

### `internal/app/model.go`

Remove references to `entry.FunctionalLabel`. No other changes needed — the model passes `entry` through to `RenderEntry` unchanged.

### `go.mod`

Remove `github.com/charmbracelet/glamour` after verifying no remaining imports. Run `go mod tidy`.

---

## Testing

### `internal/api/mw_test.go`

Update test assertions that reference `entry.Definitions` or `entry.FunctionalLabel` to use `entry.DefinitionGroups`.

Add a test case verifying multi-POS grouping: a mock response with two `rawDictEntry` items with different `FL` values should produce two `PosGroup` entries.

### `internal/ui/` (no existing tests)

No new UI tests — rendering is too terminal-dependent. Visual verification via running the binary.

---

## Non-Goals

- No changes to keybinds, layout, or sidebar
- No changes to the MW API client fetch logic
- No new data fields beyond what MW already returns

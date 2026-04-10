# lazydict — Design Spec
_2026-04-10_

## Overview

`lazydict` is a terminal UI for the Merriam-Webster dictionary, written in Go using the Charm library stack (BubbleTea, Bubbles, Lipgloss, Glamour). Inspired by lazygit's navigation model: bordered panels, vim keybinds, a persistent sidebar, and a clean status bar.

---

## Architecture

**Approach:** Flat root model + `charmbracelet/bubbles` components.

A single `app.Model` struct holds all state. Pre-built Bubbles components handle the heavy lifting — `textinput.Model` for search, `viewport.Model` for the scrollable content pane, `list.Model` for History and Favorites. The root model routes keypresses to the active component and assembles views using Lipgloss.

### Project Layout

```
lazydict/
├── main.go
├── cmd/
│   └── root.go              # cobra CLI (optional: lazydict [word])
├── internal/
│   ├── app/
│   │   ├── model.go         # root BubbleTea model, Update, View
│   │   ├── keymap.go        # all keybindings in one place
│   │   └── messages.go      # custom Msg types (WordFetchedMsg, ErrMsg)
│   ├── api/
│   │   └── mw.go            # MW REST client, response structs, in-memory cache
│   ├── ui/
│   │   ├── layout.go        # lipgloss styles, border logic, split sizing
│   │   ├── search.go        # search bar view (wraps textinput.Model)
│   │   ├── wordlist.go      # history/favorites list (wraps list.Model)
│   │   └── content.go       # right pane (wraps viewport.Model)
│   └── store/
│       └── store.go         # history + bookmarks persistence
└── config/
    └── config.go            # API key from env or config file
```

### Dependencies

| Package | Purpose |
|---|---|
| `github.com/charmbracelet/bubbletea` | TUI framework |
| `github.com/charmbracelet/bubbles` | textinput, viewport, list, spinner |
| `github.com/charmbracelet/lipgloss` | borders, colors, layout |
| `github.com/charmbracelet/glamour` | Markdown rendering for definitions |
| `github.com/spf13/cobra` | CLI entrypoint |

---

## UI Layout

Fixed **25% / 75%** split. Active pane gets a highlighted (blue) border; inactive pane gets a dimmed border.

```
┌─ SEARCH ──────────┐ ┌─────────────────────────────────────────────────┐
│ record█           │ │ record   noun | verb   ˈre-kərd / ri-ˈkȯrd       │
└───────────────────┘ │                                                   │
┌─ HISTORY ─────────┐ │ DEFINITIONS                                       │
│ ▶ record          │ │ 1. a thing constituting a piece of evidence…      │
│   ephemeral       │ │ 2. the best performance or most remarkable…       │
│   luminous        │ │                                                   │
│   serendipity     │ │ SYNONYMS                                          │
│   pernicious      │ │ account, chronicle, document, register…           │
└───────────────────┘ │                                                   │
┌─ FAVORITES ───────┐ │ EXAMPLES                                          │
│   ephemeral       │ │ "She kept a careful record of expenses."          │
│   luminous        │ │                                                   │
│   pernicious      │ │ FORMS                                             │
└───────────────────┘ │ recorded, recording, records                      │
                      │                                                   │
                      │ ETYMOLOGY                                         │
                      │ Middle English, from Anglo-French recorder…       │
                      │                                                   │
                      │ PRONUNCIATION                                      │
                      │ noun: ˈre-kərd   verb: ri-ˈkȯrd                  │
                      └───────────────────────────────────────────────────┘
 / search  j/k navigate  h/l switch section  Tab focus  Shift-j/k scroll  b bookmark  q quit
```

### Left Pane Sections

Three stacked bordered sections within the left pane:
1. **SEARCH** — `textinput.Model`, Enter submits
2. **HISTORY** — `list.Model`, most recent at top, max 100 entries
3. **FAVORITES** — `list.Model`, add-order preserved

### Right Pane

Single `viewport.Model` rendering all content sections stacked:
**Definitions → Synonyms → Examples → Forms → Etymology → Pronunciation**

Rendered via Glamour (Markdown) for clean typography. Scrollbar rendered on the right edge of the viewport.

---

## Keybindings

| Key | Action |
|---|---|
| `Tab` / `Shift-Tab` | Toggle focus: left pane ↔ right pane |
| `h` / `l` | Cycle left pane sections: Search → History → Favorites (only when search input is not in typing mode) |
| `j` / `k` | Navigate items in History or Favorites |
| `Enter` | Submit search (in Search section) / select item (in History or Favorites) |
| `Esc` | Exit search typing mode without submitting; returns to nav mode |
| `/` | Jump focus to Search section and enter typing mode from anywhere |
| `Shift-j` / `Shift-k` | Scroll right content pane (works from either pane, except when typing in search) |
| `b` | Bookmark / unbookmark current word |
| `d` | Delete highlighted item from History or Favorites (no-op in Search section) |
| `q` / `Ctrl-c` | Quit (no-op when search input is in typing mode — must Esc first) |

**Typing mode:** When the Search section is active and the user presses `/` or `Enter` on it, the `textinput` captures all keystrokes. `h`/`l`/`j`/`k` type into the input. Press `Esc` to return to nav mode without searching, or `Enter` to submit.

---

## Data Flow & API

On search submit, the model dispatches a `tea.Cmd` that calls:
- **MW Dictionary API** — definitions, forms, pronunciation, etymology
- **MW Thesaurus API** — synonyms, antonyms

Both calls run concurrently. On completion, a `WordFetchedMsg` carries the combined `Entry` into `Update`. On failure, an `ErrMsg` renders an error block in the right pane.

### Internal Entry Struct

```go
type Entry struct {
    Word             string
    Pronunciation    string
    FunctionalLabel  string
    Definitions      []Definition
    Synonyms         []string
    Examples         []string
    Forms            []string
    Etymology        string
}
```

**In-memory cache:** `map[string]Entry` on the model. Re-fetches on app restart. History stores word strings only, not full entries.

**Startup word:** `lazydict record` passes `"record"` as initial search, fires lookup before first render.

---

## State & Persistence

Persisted to `~/.config/lazydict/data.json`:

```json
{
  "history": ["record", "ephemeral", "luminous"],
  "favorites": ["ephemeral", "luminous"]
}
```

- **History:** capped deque, max 100 entries, newest first. Duplicate lookups bubble word to top.
- **Favorites:** ordered list (add-order preserved). `b` toggles — adds if absent, removes if present.
- `d` removes highlighted item from active list.
- File read once at startup, written synchronously on every mutation.

**Config** at `~/.config/lazydict/config.toml`:

```toml
mw_key = "your-key-here"
```

`$LAZYDICT_MW_KEY` env var takes precedence. If neither is set, app exits with:
```
lazydict: MW API key not set. Set $LAZYDICT_MW_KEY or add mw_key to ~/.config/lazydict/config.toml
```

---

## Error Handling

| Scenario | Behavior |
|---|---|
| Network failure / API error | Styled error block in right pane, left pane stays interactive |
| Word not found (404) | "No results for «word»" message in right pane |
| Missing API key | Fatal error at startup, message to stderr |
| Malformed API response | Generic error in pane; details logged if `$LAZYDICT_DEBUG=1` |
| Empty search | Ignored, no API call fired |

---

## Testing

| Package | What's tested |
|---|---|
| `internal/api` | Unit tests with `httptest.NewServer`: happy path, 404, malformed JSON |
| `internal/store` | Unit tests: read/write, dedup, cap enforcement, bookmark toggle |
| `internal/app` | BubbleTea model tests via `teatest`: key routing, focus switching, search submit |

No UI snapshot tests — terminal rendering is too environment-dependent.

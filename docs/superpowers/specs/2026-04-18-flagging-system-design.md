# Flagging System & UX Rework — Design Spec
_2026-04-18_

## 1. Overview

Add a persistent word-flagging system (section 4) so users can mark words whose parsed output is broken, capture a definition snapshot and a note, and give agents a clean file to batch-repair from. Simultaneously rework insert mode to be vim-style and context-aware, add a `c` shortcut to clear-and-search, and fix a history deduplication bug.

---

## 2. Data & Storage

### 2.1 Flag entry schema

```json
{
  "word": "ephemeral",
  "snapshot": "<plain-text rendered definition as shown in TUI>",
  "note": "definition #2 is garbled — raw MW markup leaking",
  "created_at": "2026-04-18T14:32:00Z"
}
```

- `snapshot` is the string returned by `ui.RenderEntry` at flag time — captured from `m.content.View()` (already rendered, no re-render needed).
- `note` is free-form user text, editable at any time, empty string until the user writes one.
- Flagging an already-flagged word refreshes `snapshot` and `created_at` but **preserves the existing note**.

### 2.2 FlagStore

New file: `internal/store/flags.go`

```go
type FlagEntry struct {
    Word      string    `json:"word"`
    Snapshot  string    `json:"snapshot"`
    Note      string    `json:"note"`
    CreatedAt time.Time `json:"created_at"`
}

type FlagStore struct { ... }
func NewFlagStore(path string) (*FlagStore, error)
func (fs *FlagStore) Add(word, snapshot string)      // upsert; preserves note
func (fs *FlagStore) UpdateNote(word, note string)
func (fs *FlagStore) Delete(word string)
func (fs *FlagStore) All() []FlagEntry               // stable order (created_at asc)
func (fs *FlagStore) Get(word string) (FlagEntry, bool)
```

Storage path: `~/.config/lazydict/flags.json` (sibling of `store.json`). Agent-accessible for bulk repair.

---

## 3. Insert Mode Rework

### 3.1 Replace `typingMode bool` with `insertTarget`

```go
type insertTarget int
const (
    insertNone     insertTarget = iota
    insertSearch
    insertFlagNote
)
```

`Model.typingMode bool` is removed; `Model.insertTarget insertTarget` replaces it throughout.

### 3.2 `i` — context-aware insert

Fires only when `insertTarget == insertNone`:

| Active section | Focused pane | Result |
|---|---|---|
| `sectionSearch` | any | enter `insertSearch`, focus search textinput |
| `sectionFlags` | `paneRight` | enter `insertFlagNote`, focus note textarea |
| anything else | any | no-op |

### 3.3 `c` — clear-and-insert in search

Fires only when `insertTarget == insertNone` and `activeSection == sectionSearch`: clears search value, then enters `insertSearch`.

### 3.4 `esc` — universal exit

Exits insert mode regardless of active target; blurs whichever component is focused. Behavior unchanged from today.

### 3.5 Status bar

Shows context-sensitive hints:
- In `insertSearch`: `esc cancel  enter search`
- In `insertFlagNote`: `esc done`
- Normal mode: existing hints + `f flag  4 flags`

---

## 4. Flags Section (Section 4)

### 4.1 Left pane

- New `sectionFlags` constant (iota value 3 → displays as `[4]`).
- `list.Model` of flagged words, same rendering as History/Favorites.
- Border label: `[4] Flags`.
- Left pane height split: search=3 rows fixed; remaining divided ~40% history / 30% favorites / 30% flags.
- `4` keybind navigates to Flags. `h/l` cycle all four sections.

### 4.2 Keybinds in Flags section

| Key | Condition | Action |
|---|---|---|
| `f` | `currentWord != ""`, not in insert mode | upsert flag for current word |
| `d` | Flags section active, flag selected | delete selected flag from FlagStore |
| `i` | Flags active, right pane focused, not in insert mode | enter `insertFlagNote` |
| `tab` | right pane focused | return focus to left pane |
| `enter` | Flags section active, flag selected | load that word's entry AND switch `activeSection` to `sectionHistory` so content view takes over |

`f` is also valid from any section/pane as long as `currentWord != ""`.

### 4.3 Right pane — flag detail view

Shown when `activeSection == sectionFlags` and a flag is selected. Replaces normal content view.

Layout (top-to-bottom inside right pane border):

```
╭─[snapshot]──────────────────────────╮
│  <read-only viewport, ~60% height>  │
╰─────────────────────────────────────╯
╭─[note]──────────────────────────────╮
│  <textarea, remaining height>        │
│  active blue border in insertMode    │
╰─────────────────────────────────────╯
```

- Snapshot viewport: scrollable with `J/K`, read-only.
- Note textarea: charmbracelet `textarea` bubble. Inactive border when `insertNone`; blue border when `insertFlagNote`. Auto-saves note to FlagStore on `esc` (exit insert).
- When no flag is selected (empty list): right pane shows placeholder text `"No flags yet. Press f to flag the current word."`.
- Switching away from Flags section restores the normal content view.

---

## 5. History Deduplication Fix

In `handleKey`, the `Submit` cache-hit branch currently skips history tracking:

```go
// current (broken)
if cached, ok := m.cache[word]; ok {
    m.entry = cached
    m.currentWord = word
    m.content.SetContent(ui.RenderEntry(cached, m.content.Width))
    m.content.GotoTop()
}
```

Fix: call `m.store.AddHistory(word)` and refresh list + suggestions after loading from cache, identical to `WordFetchedMsg`:

```go
// fixed
if cached, ok := m.cache[word]; ok {
    m.entry = cached
    m.currentWord = word
    m.store.AddHistory(word)
    ui.SetWords(&m.history, m.store.History())
    m.search.SetSuggestions(m.store.History())
    m.content.SetContent(ui.RenderEntry(cached, m.content.Width))
    m.content.GotoTop()
}
```

---

## 6. File Inventory

| File | Change |
|---|---|
| `internal/store/flags.go` | New — FlagStore |
| `internal/store/flags_test.go` | New — FlagStore unit tests |
| `internal/app/model.go` | Replace `typingMode` with `insertTarget`; add `flagStore`, `flagNote textarea`, `flagSnapshotVP viewport`; new render methods; history fix |
| `internal/app/keymap.go` | Add `Flag`, `ClearSearch`, `Section4` bindings |
| `internal/app/messages.go` | No change expected |
| `internal/ui/layout.go` | Adjust left pane height split for 4 sections |
| `cmd/root.go` | Wire up FlagStore (load path, pass to Model) |
| `config/config.go` | Add `FlagsPath()` helper (or derive in root.go) |

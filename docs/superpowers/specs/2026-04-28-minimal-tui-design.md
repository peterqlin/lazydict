# Minimal TUI — Design Spec

**Branch:** `feat/minimal-tui` (experimental, not intended to merge)
**Date:** 2026-04-28

## Goal

Strip lazydict down to two components: a search bar and a results box. No sections, no panes, no modes, no legend. Autofill from history survives.

## Layout

```
╭──────────────────────────────╮
│ search input (autofill)      │
╰──────────────────────────────╯
╭──────────────────────────────╮
│                              │
│  results / welcome / error   │
│  (mouse-scrollable)          │
│                              │
╰──────────────────────────────╯
```

- Search box: fixed height 3 rows, full terminal width, rounded border, no label.
- Results box: fills remaining height, full terminal width, rounded border, no label.
- No status bar.
- No left panel.

## Model Struct

Keep:
- `search textinput.Model` — always focused; `ShowSuggestions: true`
- `content viewport.Model` — results display; mouse-scrollable
- `spin spinner.Model`
- `entry *api.Entry`, `cache map[string]*api.Entry`
- `store *store.Store` — loaded for autofill suggestions only; history/favorites lists never rendered
- `client *api.Client`, `cfg *config.Config`
- `width`, `height int`, `loading bool`, `err string`, `currentWord string`

Remove:
- `focused`, `activeSection`, `insertTarget`
- `history list.Model`, `favorites list.Model`, `flags list.Model`
- `flagSnap viewport.Model`, `flagNote textarea.Model`
- `flagStore *store.FlagStore`
- `keys KeyMap`

## Keybinds

| Key | Action |
|-----|--------|
| `enter` | submit search (trim whitespace; no-op if empty) |
| `esc` / `ctrl+c` | quit |
| mouse scroll | scroll results viewport |

`q` types into the search box — no quit binding on `q`.

No `i`, `tab`, `j/k`, `J/K`, `1-4`, `b`, `f`, `d`, `c` bindings.

## Files Changed

| File | Change |
|------|--------|
| `internal/app/model.go` | Rewrite — minimal struct, Update, View |
| `internal/app/keymap.go` | Rewrite — only `Quit` (esc/ctrl+c) and `Submit` (enter) |
| `internal/app/messages.go` | Unchanged |
| `internal/ui/layout.go` | Unchanged |
| `cmd/root.go` | Remove `flagStore` init + arg; add `tea.WithMouseCellMotion()` |

## Behavior

- Search input is always focused. No mode switching.
- On `enter`: blur search → set loading → fetch → on result show entry, update history+autofill → re-focus search.
- On `esc`/`ctrl+c`: quit immediately.
- History store still writes (so autofill accumulates across sessions).
- Results border: `BorderInactive` style (no active/inactive distinction — only one box at a time has visual focus).
- Mouse wheel events forwarded to `content` viewport via `tea.MouseMsg`.

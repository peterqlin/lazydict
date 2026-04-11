# lazydict

A lazygit-inspired terminal UI for the [Merriam-Webster Dictionary](https://dictionaryapi.com), built with [Bubbletea](https://github.com/charmbracelet/bubbletea) and the Charm library stack.

## Features

- **Instant lookup** — search any word, get definitions, synonyms, antonyms, examples, forms, and etymology in one scrollable page
- **Persistent sidebar** — search box, lookup history (last 100), and bookmarked favorites always visible
- **Vim keybinds** — `i` to search, `h/l` to switch sections, `j/k` to navigate, `Shift-j/k` to scroll
- **Bookmarks** — press `b` to save/unsave any word, persisted across sessions
- **Concurrent API fetch** — dictionary and thesaurus APIs are called in parallel
- **CLI word arg** — `lazydict ephemeral` opens directly to that word

## Installation

```bash
go install github.com/peterqlin/lazydict@latest
```

Or build from source:

```bash
git clone https://github.com/peterqlin/lazydict.git
cd lazydict
go build -o lazydict .
```

## Setup

Get free API keys from [dictionaryapi.com](https://dictionaryapi.com) (separate keys for Dictionary and Thesaurus).

```bash
export LAZYDICT_MW_KEY=your-dictionary-key
export LAZYDICT_MW_THES_KEY=your-thesaurus-key   # optional; falls back to MW_KEY
```

Or create `~/.config/lazydict/config.toml`:

```toml
mw_key     = "your-dictionary-key"
mw_thes_key = "your-thesaurus-key"
```

## Usage

```bash
lazydict              # open with empty search
lazydict ephemeral    # open and immediately look up "ephemeral"
```

## Keybinds

| Key | Action |
|-----|--------|
| `i` | Enter search (typing mode) |
| `Enter` | Submit search / select word from list |
| `Esc` | Exit typing mode |
| `h` / `l` | Cycle left pane sections (Search → History → Favorites) |
| `j` / `k` | Navigate items in History or Favorites |
| `Tab` | Switch focus between left and right pane |
| `Shift-j` / `Shift-k` | Scroll content pane |
| `b` | Bookmark / unbookmark current word |
| `d` | Delete item from History or Favorites |
| `q` | Quit |

## Data

History and favorites are saved to `~/.config/lazydict/data.json`. History is capped at 100 entries; duplicates bubble to the top.

## Stack

- [Bubbletea](https://github.com/charmbracelet/bubbletea) — TUI framework
- [Bubbles](https://github.com/charmbracelet/bubbles) — textinput, viewport, list
- [Lipgloss](https://github.com/charmbracelet/lipgloss) — layout and styling
- [Glamour](https://github.com/charmbracelet/glamour) — Markdown rendering
- [Cobra](https://github.com/spf13/cobra) — CLI

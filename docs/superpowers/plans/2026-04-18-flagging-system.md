# Flagging System & UX Rework Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a persistent word-flagging system (section 4) with definition snapshots and editable notes, rework insert mode to be vim-style and context-aware, add `c` to clear-and-search, and fix history deduplication on cache hits.

**Architecture:** A new `FlagStore` in `internal/store/flags.go` persists flags to `~/.config/lazydict/flags.json`. The model replaces `typingMode bool` with an `insertTarget` enum to make insert mode context-aware across search and flag-note components. The right pane renders a flag detail view (snapshot viewport + note textarea) when the Flags section is active.

**Tech Stack:** Go 1.24, charmbracelet/bubbles v1.0.0 (list, viewport, textinput, textarea, spinner), bubbletea v1.3.10, lipgloss.

---

## File Map

| File | Action | Responsibility |
|---|---|---|
| `internal/store/flags.go` | Create | FlagStore: load/save/add/update/delete flags |
| `internal/store/flags_test.go` | Create | Unit tests for FlagStore |
| `internal/app/keymap.go` | Modify | Add Flag, ClearSearch, Section4 bindings |
| `internal/app/model.go` | Modify | insertTarget, sectionFlags, flag list/viewport/textarea, all key handlers, resize, render |
| `internal/app/model_test.go` | Modify | Update newTestModel (FlagStore), fix TypingMode tests, add new behaviour tests |
| `cmd/root.go` | Modify | Create FlagStore, pass to app.New |

---

## Task 1: FlagStore

**Files:**
- Create: `internal/store/flags.go`
- Create: `internal/store/flags_test.go`

- [ ] **Step 1: Write failing tests**

```go
// internal/store/flags_test.go
package store_test

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/peterqlin/lazydict/internal/store"
)

func newFlagStore(t *testing.T) *store.FlagStore {
	t.Helper()
	fs, err := store.NewFlagStore(filepath.Join(t.TempDir(), "flags.json"))
	if err != nil {
		t.Fatalf("NewFlagStore: %v", err)
	}
	return fs
}

func TestFlagAdd(t *testing.T) {
	fs := newFlagStore(t)
	fs.Add("ephemeral", "snapshot text")
	entries := fs.All()
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].Word != "ephemeral" || entries[0].Snapshot != "snapshot text" {
		t.Errorf("unexpected entry: %+v", entries[0])
	}
}

func TestFlagUpsertPreservesNote(t *testing.T) {
	fs := newFlagStore(t)
	fs.Add("ephemeral", "old snapshot")
	fs.UpdateNote("ephemeral", "my note")
	fs.Add("ephemeral", "new snapshot") // re-flag: refreshes snapshot, keeps note
	e, ok := fs.Get("ephemeral")
	if !ok {
		t.Fatal("entry not found")
	}
	if e.Snapshot != "new snapshot" {
		t.Errorf("expected new snapshot, got %q", e.Snapshot)
	}
	if e.Note != "my note" {
		t.Errorf("expected note preserved, got %q", e.Note)
	}
}

func TestFlagUpdateNote(t *testing.T) {
	fs := newFlagStore(t)
	fs.Add("ephemeral", "snap")
	fs.UpdateNote("ephemeral", "definition 2 is broken")
	e, _ := fs.Get("ephemeral")
	if e.Note != "definition 2 is broken" {
		t.Errorf("unexpected note: %q", e.Note)
	}
}

func TestFlagDelete(t *testing.T) {
	fs := newFlagStore(t)
	fs.Add("alpha", "snap1")
	fs.Add("beta", "snap2")
	fs.Delete("alpha")
	entries := fs.All()
	if len(entries) != 1 || entries[0].Word != "beta" {
		t.Errorf("unexpected entries after delete: %+v", entries)
	}
}

func TestFlagAllOrder(t *testing.T) {
	fs := newFlagStore(t)
	fs.Add("beta", "s")
	time.Sleep(time.Millisecond) // ensure distinct created_at
	fs.Add("alpha", "s")
	entries := fs.All()
	if entries[0].Word != "beta" || entries[1].Word != "alpha" {
		t.Errorf("expected created_at asc order, got %v %v", entries[0].Word, entries[1].Word)
	}
}

func TestFlagPersistence(t *testing.T) {
	path := filepath.Join(t.TempDir(), "flags.json")
	fs1, _ := store.NewFlagStore(path)
	fs1.Add("persist", "snap")
	fs1.UpdateNote("persist", "a note")

	fs2, err := store.NewFlagStore(path)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	e, ok := fs2.Get("persist")
	if !ok || e.Note != "a note" {
		t.Errorf("data not persisted: %+v, ok=%v", e, ok)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd C:/Users/230pe/Projects/lazydict && go test ./internal/store/... -run TestFlag -v
```
Expected: compilation error — `store.NewFlagStore` undefined.

- [ ] **Step 3: Implement FlagStore**

```go
// internal/store/flags.go
package store

import (
	"encoding/json"
	"fmt"
	"os"
	"slices"
	"sync"
	"time"
)

type FlagEntry struct {
	Word      string    `json:"word"`
	Snapshot  string    `json:"snapshot"`
	Note      string    `json:"note"`
	CreatedAt time.Time `json:"created_at"`
}

type FlagStore struct {
	mu      sync.Mutex
	path    string
	entries []FlagEntry
}

func NewFlagStore(path string) (*FlagStore, error) {
	fs := &FlagStore{path: path}
	b, err := os.ReadFile(path)
	if err == nil {
		_ = json.Unmarshal(b, &fs.entries)
	}
	if fs.entries == nil {
		fs.entries = []FlagEntry{}
	}
	return fs, nil
}

func (fs *FlagStore) save() {
	b, err := json.MarshalIndent(fs.entries, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "lazydict: flagstore marshal error: %v\n", err)
		return
	}
	if err := os.WriteFile(fs.path, b, 0600); err != nil {
		fmt.Fprintf(os.Stderr, "lazydict: flagstore write error: %v\n", err)
	}
}

// Add upserts a flag: refreshes snapshot and created_at, preserves existing note.
func (fs *FlagStore) Add(word, snapshot string) {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	for i, e := range fs.entries {
		if e.Word == word {
			fs.entries[i].Snapshot = snapshot
			fs.entries[i].CreatedAt = time.Now()
			fs.save()
			return
		}
	}
	fs.entries = append(fs.entries, FlagEntry{
		Word:      word,
		Snapshot:  snapshot,
		CreatedAt: time.Now(),
	})
	fs.save()
}

// UpdateNote sets the note for a flagged word. No-op if word not flagged.
func (fs *FlagStore) UpdateNote(word, note string) {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	for i, e := range fs.entries {
		if e.Word == word {
			fs.entries[i].Note = note
			fs.save()
			return
		}
	}
}

// Delete removes the flag for a word.
func (fs *FlagStore) Delete(word string) {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	fs.entries = slices.DeleteFunc(fs.entries, func(e FlagEntry) bool { return e.Word == word })
	fs.save()
}

// All returns all flags in created_at ascending order.
func (fs *FlagStore) All() []FlagEntry {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	out := make([]FlagEntry, len(fs.entries))
	copy(out, fs.entries)
	return out
}

// Get returns the flag entry for a word and whether it was found.
func (fs *FlagStore) Get(word string) (FlagEntry, bool) {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	for _, e := range fs.entries {
		if e.Word == word {
			return e, true
		}
	}
	return FlagEntry{}, false
}

// Words returns the list of flagged words in created_at ascending order.
func (fs *FlagStore) Words() []string {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	out := make([]string, len(fs.entries))
	for i, e := range fs.entries {
		out[i] = e.Word
	}
	return out
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
cd C:/Users/230pe/Projects/lazydict && go test ./internal/store/... -run TestFlag -v
```
Expected: all 6 TestFlag* tests PASS.

- [ ] **Step 5: Run full test suite to check for regressions**

```bash
cd C:/Users/230pe/Projects/lazydict && go test ./...
```
Expected: all existing tests pass.

- [ ] **Step 6: Commit**

```bash
cd C:/Users/230pe/Projects/lazydict && rtk git add internal/store/flags.go internal/store/flags_test.go && rtk git commit -m "feat: add FlagStore for persistent word flagging"
```

---

## Task 2: KeyMap Additions

**Files:**
- Modify: `internal/app/keymap.go`

- [ ] **Step 1: Add three new bindings to the KeyMap struct and DefaultKeyMap**

Replace the entire `internal/app/keymap.go` with:

```go
package app

import "github.com/charmbracelet/bubbles/key"

type KeyMap struct {
	Quit         key.Binding
	SwitchPane   key.Binding
	SectionLeft  key.Binding
	SectionRight key.Binding
	Section1     key.Binding
	Section2     key.Binding
	Section3     key.Binding
	Section4     key.Binding
	Up           key.Binding
	Down         key.Binding
	EnterTyping  key.Binding
	ExitTyping   key.Binding
	Submit       key.Binding
	ScrollUp     key.Binding
	ScrollDown   key.Binding
	Bookmark     key.Binding
	Delete       key.Binding
	Flag         key.Binding
	ClearSearch  key.Binding
}

func DefaultKeyMap() KeyMap {
	return KeyMap{
		Quit: key.NewBinding(
			key.WithKeys("q", "ctrl+c"),
			key.WithHelp("q", "quit"),
		),
		SwitchPane: key.NewBinding(
			key.WithKeys("tab", "shift+tab"),
			key.WithHelp("tab", "switch pane"),
		),
		SectionLeft: key.NewBinding(
			key.WithKeys("h"),
			key.WithHelp("h", "prev section"),
		),
		SectionRight: key.NewBinding(
			key.WithKeys("l"),
			key.WithHelp("l", "next section"),
		),
		Section1: key.NewBinding(
			key.WithKeys("1"),
			key.WithHelp("1", "search"),
		),
		Section2: key.NewBinding(
			key.WithKeys("2"),
			key.WithHelp("2", "history"),
		),
		Section3: key.NewBinding(
			key.WithKeys("3"),
			key.WithHelp("3", "favorites"),
		),
		Section4: key.NewBinding(
			key.WithKeys("4"),
			key.WithHelp("4", "flags"),
		),
		Up: key.NewBinding(
			key.WithKeys("k", "up"),
			key.WithHelp("k", "up"),
		),
		Down: key.NewBinding(
			key.WithKeys("j", "down"),
			key.WithHelp("j", "down"),
		),
		EnterTyping: key.NewBinding(
			key.WithKeys("i"),
			key.WithHelp("i", "insert"),
		),
		ExitTyping: key.NewBinding(
			key.WithKeys("esc"),
			key.WithHelp("esc", "normal"),
		),
		Submit: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "submit"),
		),
		ScrollUp: key.NewBinding(
			key.WithKeys("K", "shift+k"),
			key.WithHelp("Shift-k", "scroll up"),
		),
		ScrollDown: key.NewBinding(
			key.WithKeys("J", "shift+j"),
			key.WithHelp("Shift-j", "scroll down"),
		),
		Bookmark: key.NewBinding(
			key.WithKeys("b"),
			key.WithHelp("b", "bookmark"),
		),
		Delete: key.NewBinding(
			key.WithKeys("d"),
			key.WithHelp("d", "delete"),
		),
		Flag: key.NewBinding(
			key.WithKeys("f"),
			key.WithHelp("f", "flag"),
		),
		ClearSearch: key.NewBinding(
			key.WithKeys("c"),
			key.WithHelp("c", "clear+search"),
		),
	}
}
```

- [ ] **Step 2: Build to verify no errors**

```bash
cd C:/Users/230pe/Projects/lazydict && go build ./...
```
Expected: compiles cleanly.

- [ ] **Step 3: Commit**

```bash
cd C:/Users/230pe/Projects/lazydict && rtk git add internal/app/keymap.go && rtk git commit -m "feat: add Flag, ClearSearch, Section4 keybindings"
```

---

## Task 3: History Deduplication Fix

**Files:**
- Modify: `internal/app/model.go` (Submit handler, cache-hit branch)
- Modify: `internal/app/model_test.go` (add test)

- [ ] **Step 1: Write the failing test**

Add this test to `internal/app/model_test.go`:

```go
func TestCacheHitMovesToHistoryTop(t *testing.T) {
	cfg := &config.Config{MWKey: "test", MWThesKey: "test"}
	st, _ := store.New(filepath.Join(t.TempDir(), "data.json"))
	fs, _ := store.NewFlagStore(filepath.Join(t.TempDir(), "flags.json"))

	// Pre-populate history and cache via WordFetchedMsg
	m := app.New(cfg, st, fs, "")
	entry := &api.Entry{}
	m2, _ := m.Update(app.WordFetchedMsg{Word: "alpha", Entry: entry})
	m3, _ := m2.(app.Model).Update(app.WordFetchedMsg{Word: "beta", Entry: entry})
	// history is now: ["beta", "alpha"]

	// Exit typing mode, navigate to History section, select "alpha" (index 1)
	m4, _ := m3.(app.Model).Update(tea.KeyMsg{Type: tea.KeyEsc})
	m5, _ := m4.(app.Model).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'2'}})
	m6, _ := m5.(app.Model).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}}) // cursor to "alpha"
	m7, _ := m6.(app.Model).Update(tea.KeyMsg{Type: tea.KeyEnter})                      // load from cache
	_ = m7

	// History should now be: ["alpha", "beta"]
	hist := st.History()
	if hist[0] != "alpha" {
		t.Errorf("expected alpha at top after cache hit, got %q", hist[0])
	}
}
```

Also update the existing `newTestModel` helper and `TestLaunchesInTypingMode`, `TestEscExitsTypingMode`, `TestIEntersTypingMode`, `TestQuitInNavMode`, `TestTabSwitchesPane`, `TestHLCyclesSections` — all need `app.New` to accept a FlagStore (done in Task 4 when we update the signature). **Skip running this test until Task 4 is complete.**

- [ ] **Step 2: Locate the cache-hit branch in model.go**

In `internal/app/model.go`, find the `Submit` handler (around line 273). The cache-hit branch currently reads:

```go
if cached, ok := m.cache[word]; ok {
    m.entry = cached
    m.currentWord = word
    m.content.SetContent(ui.RenderEntry(cached, m.content.Width))
    m.content.GotoTop()
}
```

- [ ] **Step 3: Apply the fix**

Replace that block with:

```go
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

- [ ] **Step 4: Build to verify no compilation errors**

```bash
cd C:/Users/230pe/Projects/lazydict && go build ./...
```

- [ ] **Step 5: Commit**

```bash
cd C:/Users/230pe/Projects/lazydict && rtk git add internal/app/model.go && rtk git commit -m "fix: move word to top of history on cache-hit selection"
```

---

## Task 4: insertTarget Enum + sectionFlags + Wire FlagStore

This task replaces `typingMode bool` with `insertTarget`, adds `sectionFlags`, updates `h/l` cycling, updates `app.New`, and updates the test helpers. All model.go and model_test.go changes land here.

**Files:**
- Modify: `internal/app/model.go`
- Modify: `internal/app/model_test.go`
- Modify: `cmd/root.go`

- [ ] **Step 1: Write updated/new tests**

Replace the entire `internal/app/model_test.go` with:

```go
package app_test

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/exp/teatest"

	"github.com/peterqlin/lazydict/config"
	"github.com/peterqlin/lazydict/internal/api"
	"github.com/peterqlin/lazydict/internal/app"
	"github.com/peterqlin/lazydict/internal/store"
)

func newTestModel(t *testing.T) app.Model {
	t.Helper()
	cfg := &config.Config{MWKey: "test", MWThesKey: "test"}
	st, err := store.New(filepath.Join(t.TempDir(), "data.json"))
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	fs, err := store.NewFlagStore(filepath.Join(t.TempDir(), "flags.json"))
	if err != nil {
		t.Fatalf("flagstore: %v", err)
	}
	return app.New(cfg, st, fs, "")
}

func TestLaunchesInTypingMode(t *testing.T) {
	m := newTestModel(t)
	if !m.TypingMode() {
		t.Error("expected typing mode on launch")
	}
}

func TestEscExitsTypingMode(t *testing.T) {
	m := newTestModel(t)
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m2 := updated.(app.Model)
	if m2.TypingMode() {
		t.Error("expected typing mode to be off after Esc")
	}
}

func TestIEntersTypingModeWhenSearchActive(t *testing.T) {
	m := newTestModel(t)
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m2 := updated.(app.Model)
	updated2, _ := m2.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'i'}})
	m3 := updated2.(app.Model)
	if !m3.TypingMode() {
		t.Error("expected typing mode after pressing i with search active")
	}
}

func TestIIsNoopWhenHistoryActive(t *testing.T) {
	m := newTestModel(t)
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m2 := updated.(app.Model)
	// navigate to History
	updated2, _ := m2.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}})
	m3 := updated2.(app.Model)
	// press i — should be no-op
	updated3, _ := m3.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'i'}})
	m4 := updated3.(app.Model)
	if m4.TypingMode() {
		t.Error("expected i to be no-op when history section active")
	}
}

func TestClearSearchEntersTypingMode(t *testing.T) {
	m := newTestModel(t)
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m2 := updated.(app.Model)
	// activeSection is search; press c
	updated2, _ := m2.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	m3 := updated2.(app.Model)
	if !m3.TypingMode() {
		t.Error("expected c to enter typing mode in search section")
	}
}

func TestQuitInNavMode(t *testing.T) {
	m := newTestModel(t)
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m2 := updated.(app.Model)
	tm := teatest.NewTestModel(t, m2, teatest.WithInitialTermSize(120, 40))
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	tm.WaitFinished(t, teatest.WithFinalTimeout(3*time.Second))
}

func TestTabSwitchesPane(t *testing.T) {
	m := newTestModel(t)
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m2 := updated.(app.Model)
	if m2.FocusedPane() != app.PaneLeft {
		t.Fatal("expected left pane focus initially")
	}
	updated2, _ := m2.Update(tea.KeyMsg{Type: tea.KeyTab})
	m3 := updated2.(app.Model)
	if m3.FocusedPane() != app.PaneRight {
		t.Error("expected right pane focus after Tab")
	}
}

func TestHLCyclesFourSections(t *testing.T) {
	m := newTestModel(t)
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m2 := updated.(app.Model)

	// search(0) → l → history(1)
	u, _ := m2.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}})
	if u.(app.Model).ActiveSection() != app.SectionHistory {
		t.Errorf("expected History after l from Search, got %v", u.(app.Model).ActiveSection())
	}
	// history(1) → l → favorites(2)
	u, _ = u.(app.Model).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}})
	if u.(app.Model).ActiveSection() != app.SectionFavorites {
		t.Errorf("expected Favorites after l, got %v", u.(app.Model).ActiveSection())
	}
	// favorites(2) → l → flags(3)
	u, _ = u.(app.Model).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}})
	if u.(app.Model).ActiveSection() != app.SectionFlags {
		t.Errorf("expected Flags after l from Favorites, got %v", u.(app.Model).ActiveSection())
	}
	// flags(3) → l → wraps to search(0)
	u, _ = u.(app.Model).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}})
	if u.(app.Model).ActiveSection() != app.SectionSearch {
		t.Errorf("expected Search after l from Flags (wrap), got %v", u.(app.Model).ActiveSection())
	}
	// search(0) → h → wraps to flags(3)
	u, _ = u.(app.Model).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h'}})
	if u.(app.Model).ActiveSection() != app.SectionFlags {
		t.Errorf("expected Flags after h from Search (wrap), got %v", u.(app.Model).ActiveSection())
	}
}

func TestCacheHitMovesToHistoryTop(t *testing.T) {
	cfg := &config.Config{MWKey: "test", MWThesKey: "test"}
	st, _ := store.New(filepath.Join(t.TempDir(), "data.json"))
	fs, _ := store.NewFlagStore(filepath.Join(t.TempDir(), "flags.json"))

	m := app.New(cfg, st, fs, "")
	entry := &api.Entry{}
	m2, _ := m.Update(app.WordFetchedMsg{Word: "alpha", Entry: entry})
	m3, _ := m2.(app.Model).Update(app.WordFetchedMsg{Word: "beta", Entry: entry})

	m4, _ := m3.(app.Model).Update(tea.KeyMsg{Type: tea.KeyEsc})
	m5, _ := m4.(app.Model).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'2'}})
	m6, _ := m5.(app.Model).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	_, _ = m6.(app.Model).Update(tea.KeyMsg{Type: tea.KeyEnter})

	hist := st.History()
	if hist[0] != "alpha" {
		t.Errorf("expected alpha at top after cache hit, got %q", hist[0])
	}
}

// Silence unused import if textarea is not directly referenced in tests.
var _ = textarea.New
```

- [ ] **Step 2: Run tests — expect compilation failure on `app.New` signature mismatch**

```bash
cd C:/Users/230pe/Projects/lazydict && go test ./internal/app/... -v 2>&1 | head -20
```
Expected: compile error — `app.New` has wrong number of arguments.

- [ ] **Step 3: Update model.go — replace typingMode, add sectionFlags, add FlagStore field, update New**

Apply these changes to `internal/app/model.go`:

**3a. Add `insertTarget` type and constants** — replace the existing `pane`/`section` block (lines 21–34) with:

```go
type pane int

const (
	paneLeft pane = iota
	paneRight
)

type section int

const (
	sectionSearch section = iota
	sectionHistory
	sectionFavorites
	sectionFlags
)

type insertTarget int

const (
	insertNone     insertTarget = iota
	insertSearch
	insertFlagNote
)
```

**3b. Update Model struct** — replace `typingMode bool` with `insertTarget insertTarget` and add `flagStore *store.FlagStore`:

```go
type Model struct {
	width  int
	height int

	focused       pane
	activeSection section
	insertTarget  insertTarget

	search    textinput.Model
	history   list.Model
	favorites list.Model
	flags     list.Model
	content   viewport.Model
	spin      spinner.Model

	entry       *api.Entry
	cache       map[string]*api.Entry
	store       *store.Store
	flagStore   *store.FlagStore
	cfg         *config.Config
	client      *api.Client

	loading     bool
	err         string
	currentWord string

	keys KeyMap
}
```

**3c. Update exported accessors** — replace the accessor block:

```go
func (m Model) TypingMode() bool       { return m.insertTarget != insertNone }
func (m Model) FocusedPane() pane      { return m.focused }
func (m Model) ActiveSection() section { return m.activeSection }
```

**3d. Update exported constants** — add `SectionFlags`:

```go
const (
	PaneLeft  = paneLeft
	PaneRight = paneRight

	SectionSearch    = sectionSearch
	SectionHistory   = sectionHistory
	SectionFavorites = sectionFavorites
	SectionFlags     = sectionFlags
)
```

**3e. Update `New` signature and body**:

```go
func New(cfg *config.Config, st *store.Store, fs *store.FlagStore, initialWord string) Model {
	ti := textinput.New()
	ti.Placeholder = "search…"
	ti.CharLimit = 100
	ti.ShowSuggestions = true
	ti.Focus()

	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(ui.ColorAccent)

	m := Model{
		focused:      paneLeft,
		activeSection: sectionSearch,
		insertTarget: insertSearch,
		search:       ti,
		spin:         sp,
		cache:        make(map[string]*api.Entry),
		store:        st,
		flagStore:    fs,
		cfg:          cfg,
		client:       api.NewClient(cfg.MWKey, cfg.MWThesKey),
		keys:         DefaultKeyMap(),
	}

	m.history = ui.NewWordList(st.History(), 0, 0)
	m.favorites = ui.NewWordList(st.Favorites(), 0, 0)
	m.flags = ui.NewWordList(fs.Words(), 0, 0)
	m.search.SetSuggestions(st.History())
	m.content = viewport.New(0, 0)
	m.content.SetContent(ui.RenderWelcome())

	if initialWord != "" {
		m.search.SetValue(initialWord)
	}

	return m
}
```

**3f. Update `Init`** — replace `textinput.Blink` guard using `typingMode` with `insertTarget`:

```go
func (m Model) Init() tea.Cmd {
	cmds := []tea.Cmd{textinput.Blink}
	if m.search.Value() != "" {
		cmds = append(cmds, m.doFetch(m.search.Value()), m.spin.Tick)
	}
	return tea.Batch(cmds...)
}
```
(No change needed here — Blink always fires on launch, same as before.)

**3g. Update `Update`** — replace the `wasTyping` guard block:

```go
case tea.KeyMsg:
    wasInSearch := m.insertTarget == insertSearch
    cmd := m.handleKey(msg)
    cmds = append(cmds, cmd)
    if wasInSearch {
        var tiCmd tea.Cmd
        m.search, tiCmd = m.search.Update(msg)
        cmds = append(cmds, tiCmd)
    }
```

**3h. Update `handleKey`** — replace the entire function:

```go
func (m *Model) handleKey(msg tea.KeyMsg) tea.Cmd {
	switch m.insertTarget {
	case insertSearch:
		switch {
		case key.Matches(msg, m.keys.ExitTyping):
			m.insertTarget = insertNone
			m.search.Blur()
		case key.Matches(msg, m.keys.Submit):
			word := strings.TrimSpace(m.search.Value())
			if word != "" {
				m.insertTarget = insertNone
				m.search.Blur()
				m.loading = true
				m.content.SetContent(fmt.Sprintf("Looking up %q…", word))
				return tea.Batch(m.doFetch(word), m.spin.Tick)
			}
		}
		return nil

	case insertFlagNote:
		if key.Matches(msg, m.keys.ExitTyping) {
			m.insertTarget = insertNone
			// note saved on esc — handled in Task 8
		}
		return nil
	}

	// insertNone — normal navigation
	switch {
	case key.Matches(msg, m.keys.Quit):
		return tea.Quit

	case key.Matches(msg, m.keys.SwitchPane):
		if m.focused == paneLeft {
			m.focused = paneRight
		} else {
			m.focused = paneLeft
		}

	case key.Matches(msg, m.keys.EnterTyping):
		switch {
		case m.activeSection == sectionSearch:
			m.focused = paneLeft
			m.insertTarget = insertSearch
			m.search.Focus()
			return textinput.Blink
		case m.activeSection == sectionFlags && m.focused == paneRight:
			// insertFlagNote handled in Task 8
		}

	case key.Matches(msg, m.keys.ClearSearch) && m.activeSection == sectionSearch:
		m.search.SetValue("")
		m.focused = paneLeft
		m.insertTarget = insertSearch
		m.search.Focus()
		return textinput.Blink

	case key.Matches(msg, m.keys.SectionLeft) && m.focused == paneLeft:
		m.activeSection = (m.activeSection + 3) % 4

	case key.Matches(msg, m.keys.SectionRight) && m.focused == paneLeft:
		m.activeSection = (m.activeSection + 1) % 4

	case key.Matches(msg, m.keys.Section1) && m.focused == paneLeft:
		m.activeSection = sectionSearch

	case key.Matches(msg, m.keys.Section2) && m.focused == paneLeft:
		m.activeSection = sectionHistory

	case key.Matches(msg, m.keys.Section3) && m.focused == paneLeft:
		m.activeSection = sectionFavorites

	case key.Matches(msg, m.keys.Section4) && m.focused == paneLeft:
		m.activeSection = sectionFlags

	case key.Matches(msg, m.keys.Up) && m.focused == paneLeft:
		switch m.activeSection {
		case sectionHistory:
			m.history.CursorUp()
		case sectionFavorites:
			m.favorites.CursorUp()
		case sectionFlags:
			m.flags.CursorUp()
		}

	case key.Matches(msg, m.keys.Down) && m.focused == paneLeft:
		switch m.activeSection {
		case sectionHistory:
			m.history.CursorDown()
		case sectionFavorites:
			m.favorites.CursorDown()
		case sectionFlags:
			m.flags.CursorDown()
		}

	case key.Matches(msg, m.keys.Submit) && m.focused == paneLeft:
		var word string
		switch m.activeSection {
		case sectionHistory:
			word = ui.SelectedWord(m.history)
		case sectionFavorites:
			word = ui.SelectedWord(m.favorites)
		case sectionFlags:
			word = ui.SelectedWord(m.flags)
		}
		if word != "" {
			if cached, ok := m.cache[word]; ok {
				m.entry = cached
				m.currentWord = word
				m.store.AddHistory(word)
				ui.SetWords(&m.history, m.store.History())
				m.search.SetSuggestions(m.store.History())
				m.content.SetContent(ui.RenderEntry(cached, m.content.Width))
				m.content.GotoTop()
				if m.activeSection == sectionFlags {
					m.activeSection = sectionHistory
				}
			} else {
				m.loading = true
				if m.activeSection == sectionFlags {
					m.activeSection = sectionHistory
				}
				return tea.Batch(m.doFetch(word), m.spin.Tick)
			}
		}

	case key.Matches(msg, m.keys.ScrollDown):
		m.content.ScrollDown(3)

	case key.Matches(msg, m.keys.ScrollUp):
		m.content.ScrollUp(3)

	case key.Matches(msg, m.keys.Bookmark) && m.currentWord != "":
		m.store.ToggleFavorite(m.currentWord)
		ui.SetWords(&m.favorites, m.store.Favorites())

	case key.Matches(msg, m.keys.Delete) && m.focused == paneLeft:
		switch m.activeSection {
		case sectionSearch:
			m.search.SetValue("")
		case sectionHistory:
			if word := ui.SelectedWord(m.history); word != "" {
				m.store.RemoveHistory(word)
				ui.SetWords(&m.history, m.store.History())
			}
		case sectionFavorites:
			if word := ui.SelectedWord(m.favorites); word != "" {
				m.store.RemoveFavorite(word)
				ui.SetWords(&m.favorites, m.store.Favorites())
			}
		case sectionFlags:
			// flag deletion handled in Task 8
		}

	case key.Matches(msg, m.keys.Flag) && m.currentWord != "":
		// flag action handled in Task 7
	}

	return nil
}
```

**3i. Update `resize`** — add flags list sizing, split remaining height four ways:

```go
func (m Model) resize() Model {
	leftW := ui.LeftPanelWidth(m.width)
	rightW := ui.RightPanelWidth(m.width, leftW)

	statusH := 1
	innerH := m.height - statusH

	searchH := 3
	remaining := innerH - searchH
	historyH := remaining * 4 / 10
	favoritesH := remaining * 3 / 10
	flagsH := remaining - historyH - favoritesH

	listInnerW := leftW - 2
	rightInnerW := rightW - 2
	rightInnerH := innerH - 2

	m.search.Width = listInnerW - 2
	m.history.SetSize(listInnerW, historyH-2)
	m.favorites.SetSize(listInnerW, favoritesH-2)
	m.flags.SetSize(listInnerW, flagsH-2)
	m.content.Width = rightInnerW
	m.content.Height = rightInnerH

	if m.entry != nil {
		m.content.SetContent(ui.RenderEntry(m.entry, rightInnerW))
	}

	return m
}
```

**3j. Update `View`** — add flags section to left pane:

```go
func (m Model) View() string {
	if m.width == 0 {
		return ""
	}

	leftW := ui.LeftPanelWidth(m.width)

	searchView := m.renderSearch(leftW)
	historyView := m.renderWordList(m.history, "History", sectionHistory, leftW)
	favoritesView := m.renderWordList(m.favorites, "Favorites", sectionFavorites, leftW)
	flagsView := m.renderWordList(m.flags, "Flags", sectionFlags, leftW)
	leftPane := lipgloss.JoinVertical(lipgloss.Left, searchView, historyView, favoritesView, flagsView)

	rightPane := m.renderContent()

	main := lipgloss.JoinHorizontal(lipgloss.Top, leftPane, " ", rightPane)
	status := m.renderStatusBar()

	return lipgloss.JoinVertical(lipgloss.Left, main, status)
}
```

- [ ] **Step 4: Update cmd/root.go to pass FlagStore to app.New**

Replace the `run` function body in `cmd/root.go`:

```go
func run(cmd *cobra.Command, args []string) error {
	cfgPath := config.DefaultPath()
	cfg, err := config.Load(cfgPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	dir := filepath.Dir(cfgPath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}

	st, err := store.New(filepath.Join(dir, "data.json"))
	if err != nil {
		return fmt.Errorf("open store: %w", err)
	}

	fs, err := store.NewFlagStore(filepath.Join(dir, "flags.json"))
	if err != nil {
		return fmt.Errorf("open flag store: %w", err)
	}

	initialWord := ""
	if len(args) > 0 {
		initialWord = args[0]
	}

	m := app.New(cfg, st, fs, initialWord)
	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("TUI error: %w", err)
	}
	return nil
}
```

- [ ] **Step 5: Run all tests**

```bash
cd C:/Users/230pe/Projects/lazydict && go test ./...
```
Expected: all tests pass. (`TestHLCyclesFourSections` now exercises 4 sections; `TestIIsNoopWhenHistoryActive` and `TestClearSearchEntersTypingMode` pass.)

- [ ] **Step 6: Commit**

```bash
cd C:/Users/230pe/Projects/lazydict && rtk git add internal/app/model.go internal/app/model_test.go cmd/root.go && rtk git commit -m "feat: replace typingMode with insertTarget, add sectionFlags, wire FlagStore"
```

---

## Task 5: `f` Keybind — Flag Current Word

**Files:**
- Modify: `internal/app/model.go` (Flag handler stub → real implementation)

- [ ] **Step 1: Write a test**

Add to `internal/app/model_test.go`:

```go
func TestFlagCurrentWord(t *testing.T) {
	cfg := &config.Config{MWKey: "test", MWThesKey: "test"}
	st, _ := store.New(filepath.Join(t.TempDir(), "data.json"))
	fs, _ := store.NewFlagStore(filepath.Join(t.TempDir(), "flags.json"))
	m := app.New(cfg, st, fs, "")

	// Simulate a word being loaded
	entry := &api.Entry{}
	m2, _ := m.Update(app.WordFetchedMsg{Word: "ephemeral", Entry: entry})
	m3, _ := m2.(app.Model).Update(tea.KeyMsg{Type: tea.KeyEsc})

	// Press f to flag
	_, _ = m3.(app.Model).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'f'}})

	entries := fs.All()
	if len(entries) != 1 || entries[0].Word != "ephemeral" {
		t.Errorf("expected ephemeral to be flagged, got %+v", entries)
	}
}
```

- [ ] **Step 2: Run to verify it fails**

```bash
cd C:/Users/230pe/Projects/lazydict && go test ./internal/app/... -run TestFlagCurrentWord -v
```
Expected: FAIL — flag entry count is 0.

- [ ] **Step 3: Implement the Flag handler**

In `handleKey`, find the stub:
```go
case key.Matches(msg, m.keys.Flag) && m.currentWord != "":
    // flag action handled in Task 7
```

Replace with:
```go
case key.Matches(msg, m.keys.Flag) && m.currentWord != "":
    m.flagStore.Add(m.currentWord, m.content.View())
    ui.SetWords(&m.flags, m.flagStore.Words())
```

- [ ] **Step 4: Run test to verify it passes**

```bash
cd C:/Users/230pe/Projects/lazydict && go test ./internal/app/... -run TestFlagCurrentWord -v
```
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
cd C:/Users/230pe/Projects/lazydict && rtk git add internal/app/model.go internal/app/model_test.go && rtk git commit -m "feat: f keybind flags current word with definition snapshot"
```

---

## Task 6: Flag Detail View (Right Pane)

This task adds the snapshot viewport + note textarea, renders the flag detail view when the Flags section is active, wires `insertFlagNote` for `i`, saves the note on `esc`, and handles `d` to delete a flag.

**Files:**
- Modify: `internal/app/model.go`

- [ ] **Step 1: Add textarea and snapshot viewport fields to Model**

In the Model struct, add after `content viewport.Model`:

```go
flagSnap  viewport.Model
flagNote  textarea.Model
```

Add the import `"github.com/charmbracelet/bubbles/textarea"` to the import block in model.go.

- [ ] **Step 2: Initialize flagSnap and flagNote in New()**

After `m.content = viewport.New(0, 0)`, add:

```go
m.flagSnap = viewport.New(0, 0)

ta := textarea.New()
ta.Placeholder = "Describe the issue…"
ta.CharLimit = 500
ta.ShowLineNumbers = false
m.flagNote = ta
```

- [ ] **Step 3: Update resize() to size flagSnap and flagNote**

In `resize()`, after `m.content.Height = rightInnerH`, add:

```go
snapH := rightInnerH * 6 / 10
noteH := rightInnerH - snapH - 2 // subtract borders
if noteH < 2 {
    noteH = 2
}
m.flagSnap.Width = rightInnerW
m.flagSnap.Height = snapH
m.flagNote.SetWidth(rightInnerW)
m.flagNote.SetHeight(noteH)
```

- [ ] **Step 4: Add loadFlagDetail helper**

Add this method to model.go:

```go
// loadFlagDetail populates flagSnap and flagNote for the currently selected flag.
func (m *Model) loadFlagDetail() {
	word := ui.SelectedWord(m.flags)
	if word == "" {
		m.flagSnap.SetContent("")
		m.flagNote.SetValue("")
		return
	}
	if e, ok := m.flagStore.Get(word); ok {
		m.flagSnap.SetContent(e.Snapshot)
		m.flagSnap.GotoTop()
		m.flagNote.SetValue(e.Note)
	}
}
```

- [ ] **Step 5: Call loadFlagDetail when navigating flags list**

In `handleKey`, update the Up/Down cases for sectionFlags:

```go
case sectionFlags:
    m.flags.CursorUp()
    m.loadFlagDetail()
```

```go
case sectionFlags:
    m.flags.CursorDown()
    m.loadFlagDetail()
```

Also call `m.loadFlagDetail()` when switching to sectionFlags via `h/l`, `4` key. Update those cases:

```go
case key.Matches(msg, m.keys.SectionLeft) && m.focused == paneLeft:
    m.activeSection = (m.activeSection + 3) % 4
    if m.activeSection == sectionFlags {
        m.loadFlagDetail()
    }

case key.Matches(msg, m.keys.SectionRight) && m.focused == paneLeft:
    m.activeSection = (m.activeSection + 1) % 4
    if m.activeSection == sectionFlags {
        m.loadFlagDetail()
    }

case key.Matches(msg, m.keys.Section4) && m.focused == paneLeft:
    m.activeSection = sectionFlags
    m.loadFlagDetail()
```

- [ ] **Step 6: Add renderFlagDetail method**

```go
func (m Model) renderFlagDetail() string {
	rightW := ui.RightPanelWidth(m.width, ui.LeftPanelWidth(m.width))
	innerW := rightW - 2

	if ui.SelectedWord(m.flags) == "" {
		placeholder := lipgloss.NewStyle().
			Foreground(ui.ColorMuted).
			Width(innerW).
			Height(m.content.Height).
			Align(lipgloss.Center, lipgloss.Center).
			Render("No flags yet.\nPress f to flag the current word.")
		style := ui.BorderInactive().Width(innerW)
		return style.Render(placeholder)
	}

	snapActive := m.focused == paneRight && m.insertTarget == insertNone
	snapStyle := ui.BorderInactive().Width(innerW)
	if snapActive {
		snapStyle = ui.BorderActive().Width(innerW)
	}
	snapBorder := ui.BorderWithTitle(m.flagSnap.View(), "Definition Snapshot", 0, rightW, snapActive)

	noteActive := m.insertTarget == insertFlagNote
	noteStyle := ui.BorderInactive().Width(innerW)
	if noteActive {
		noteStyle = ui.BorderActive().Width(innerW)
	}
	_ = noteStyle
	noteBorder := ui.BorderWithTitle(m.flagNote.View(), "Note", 0, rightW, noteActive)

	_ = snapStyle
	return lipgloss.JoinVertical(lipgloss.Left, snapBorder, noteBorder)
}
```

- [ ] **Step 7: Update renderContent to branch on sectionFlags**

Replace the `renderContent` method:

```go
func (m Model) renderContent() string {
	if m.activeSection == sectionFlags {
		return m.renderFlagDetail()
	}

	active := m.focused == paneRight
	rightW := ui.RightPanelWidth(m.width, ui.LeftPanelWidth(m.width))

	var inner string
	if m.loading {
		inner = m.spin.View() + " Looking up…"
	} else {
		inner = m.content.View()
	}

	style := ui.BorderInactive().Width(rightW - 2)
	if active {
		style = ui.BorderActive().Width(rightW - 2)
	}
	return style.Render(inner)
}
```

- [ ] **Step 8: Wire insertFlagNote in handleKey**

Replace the stub in the `EnterTyping` case:

```go
case m.activeSection == sectionFlags && m.focused == paneRight:
    m.insertTarget = insertFlagNote
    m.flagNote.Focus()
    return textarea.Blink
```

- [ ] **Step 9: Save note and blur on esc in insertFlagNote case**

In the `insertFlagNote` branch of the top-level switch in `handleKey`:

```go
case insertFlagNote:
    if key.Matches(msg, m.keys.ExitTyping) {
        m.insertTarget = insertNone
        m.flagNote.Blur()
        if word := ui.SelectedWord(m.flags); word != "" {
            m.flagStore.UpdateNote(word, m.flagNote.Value())
        }
    }
    return nil
```

- [ ] **Step 10: Forward key events to flagNote textarea when in insertFlagNote**

In `Update`, update the key forwarding block to also handle textarea:

```go
case tea.KeyMsg:
    wasInSearch := m.insertTarget == insertSearch
    wasInFlagNote := m.insertTarget == insertFlagNote
    cmd := m.handleKey(msg)
    cmds = append(cmds, cmd)
    if wasInSearch {
        var tiCmd tea.Cmd
        m.search, tiCmd = m.search.Update(msg)
        cmds = append(cmds, tiCmd)
    }
    if wasInFlagNote {
        var taCmd tea.Cmd
        m.flagNote, taCmd = m.flagNote.Update(msg)
        cmds = append(cmds, taCmd)
    }
```

- [ ] **Step 11: Implement flag deletion with `d` in sectionFlags**

Replace the sectionFlags stub in the Delete case:

```go
case sectionFlags:
    if word := ui.SelectedWord(m.flags); word != "" {
        m.flagStore.Delete(word)
        ui.SetWords(&m.flags, m.flagStore.Words())
        m.loadFlagDetail()
    }
```

- [ ] **Step 12: Build to verify no errors**

```bash
cd C:/Users/230pe/Projects/lazydict && go build ./...
```
Expected: compiles cleanly.

- [ ] **Step 13: Run all tests**

```bash
cd C:/Users/230pe/Projects/lazydict && go test ./...
```
Expected: all tests pass.

- [ ] **Step 14: Commit**

```bash
cd C:/Users/230pe/Projects/lazydict && rtk git add internal/app/model.go && rtk git commit -m "feat: flag detail view with snapshot viewport, note textarea, insertFlagNote mode"
```

---

## Task 7: Status Bar Updates

**Files:**
- Modify: `internal/app/model.go` (renderStatusBar method only)

- [ ] **Step 1: Replace renderStatusBar**

```go
func (m Model) renderStatusBar() string {
	var parts []string
	switch m.insertTarget {
	case insertSearch:
		parts = []string{
			ui.KeyHint("esc") + " normal",
			ui.KeyHint("enter") + " search",
		}
	case insertFlagNote:
		parts = []string{
			ui.KeyHint("esc") + " done",
		}
	default:
		parts = []string{
			ui.KeyHint("i") + " insert",
			ui.KeyHint("c") + " clear+search",
			ui.KeyHint("j/k") + " navigate",
			ui.KeyHint("1-4") + " section",
			ui.KeyHint("tab") + " switch pane",
			ui.KeyHint("J/K") + " scroll",
			ui.KeyHint("b") + " bookmark",
			ui.KeyHint("f") + " flag",
			ui.KeyHint("d") + " delete",
			ui.KeyHint("q") + " quit",
		}
	}
	return ui.StatusBarStyle().Render(strings.Join(parts, "  "))
}
```

- [ ] **Step 2: Build and run tests**

```bash
cd C:/Users/230pe/Projects/lazydict && go build ./... && go test ./...
```
Expected: compiles and all tests pass.

- [ ] **Step 3: Commit**

```bash
cd C:/Users/230pe/Projects/lazydict && rtk git add internal/app/model.go && rtk git commit -m "feat: update status bar hints for insert mode and flags section"
```

---

## Self-Review Notes

- **Spec §3.2**: `i` in insertNone + sectionSearch → insertSearch ✓ (Task 4 step 3h). `i` in insertNone + sectionFlags + paneRight → insertFlagNote ✓ (Task 6 step 8). `i` in any other section → no-op ✓ (EnterTyping case only matches those two conditions).
- **Spec §3.3**: `c` fires only when insertNone + sectionSearch → clears + insertSearch ✓ (Task 4 step 3h).
- **Spec §4.2**: `enter` on flag loads word AND switches to sectionHistory ✓ (Task 4 step 3h, Submit case).
- **Spec §4.3**: `d` deletes selected flag ✓ (Task 6 step 11). Note saves on esc ✓ (Task 6 step 9).
- **Spec §5**: history cache-hit fix ✓ (Task 3 + Task 4 step 3h).
- **`textarea.Blink`**: The charmbracelet/bubbles textarea package exports a `Blink` command identical to textinput. If the import path differs, replace with `textarea.Blink` → the correct cmd from that package.
- **`BorderWithTitle` with `num=0`**: The flag detail sub-panels pass `num=0`, which renders `[0]`. To suppress the number bracket for sub-panels, either pass `-1` and guard in `BorderWithTitle`, or use a simpler label-only border for the flag detail. The simplest fix is to pass a sentinel (e.g. `num=0`) and add `if num > 0` guard in `BorderWithTitle`. This is a cosmetic issue — leave it as-is or fix during implementation.

# Minimal TUI Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Strip lazydict to search bar + results box on a new experimental branch — always-on search mode, mouse-scrollable results, no labels/legend/navigation.

**Architecture:** Trim model.go directly on `feat/minimal-tui`. Remove 4-section/2-pane machinery; search always focused; esc/ctrl+c quit; all other keys type into search; mouse events forwarded to viewport. Three files change: `internal/app/keymap.go`, `internal/app/model.go`, `cmd/root.go`.

**Tech Stack:** Go 1.24, bubbletea v1.3.10, bubbles v1.0.0 (textinput, viewport, spinner), lipgloss v1.1.1

---

### Task 1: Create experimental branch

**Files:** none

- [ ] **Step 1: Create and switch to branch**

```bash
git checkout -b feat/minimal-tui
```

Expected: `Switched to a new branch 'feat/minimal-tui'`

---

### Task 2: Rewrite keymap.go

**Files:**
- Modify: `internal/app/keymap.go`

- [ ] **Step 1: Replace keymap.go**

```go
package app

import "github.com/charmbracelet/bubbles/key"

type KeyMap struct {
	Quit   key.Binding
	Submit key.Binding
}

func DefaultKeyMap() KeyMap {
	return KeyMap{
		Quit: key.NewBinding(
			key.WithKeys("esc", "ctrl+c"),
			key.WithHelp("esc", "quit"),
		),
		Submit: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "search"),
		),
	}
}
```

- [ ] **Step 2: Verify compile**

```bash
go build ./internal/app/...
```

Expected: exits 0 (test file compile errors are OK here — fixed in Task 5).

- [ ] **Step 3: Commit**

```bash
git add internal/app/keymap.go
git commit -m "feat(minimal): slim keymap to quit+submit only"
```

---

### Task 3: Rewrite model.go

**Files:**
- Modify: `internal/app/model.go`

- [ ] **Step 1: Replace model.go with minimal version**

```go
package app

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/peterqlin/lazydict/config"
	"github.com/peterqlin/lazydict/internal/api"
	"github.com/peterqlin/lazydict/internal/store"
	"github.com/peterqlin/lazydict/internal/ui"
)

// Model is the root BubbleTea model.
type Model struct {
	width  int
	height int

	search  textinput.Model
	content viewport.Model
	spin    spinner.Model

	entry       *api.Entry
	cache       map[string]*api.Entry
	store       *store.Store
	cfg         *config.Config
	client      *api.Client

	loading     bool
	err         string
	currentWord string
}

// Exported accessors for tests.
func (m Model) SearchFocused() bool { return m.search.Focused() }
func (m Model) CurrentWord() string { return m.currentWord }
func (m Model) Err() string         { return m.err }
func (m Model) Loading() bool       { return m.loading }

// New creates a new Model.
func New(cfg *config.Config, st *store.Store, initialWord string) Model {
	ti := textinput.New()
	ti.Placeholder = "search…"
	ti.CharLimit = 100
	ti.ShowSuggestions = true
	ti.Focus()

	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(ui.ColorAccent)

	m := Model{
		search: ti,
		spin:   sp,
		cache:  make(map[string]*api.Entry),
		store:  st,
		cfg:    cfg,
		client: api.NewClient(cfg.MWKey, cfg.MWThesKey),
	}

	m.search.SetSuggestions(st.History())
	m.content = viewport.New(0, 0)
	m.content.SetContent(ui.RenderWelcome())

	if initialWord != "" {
		m.search.SetValue(initialWord)
	}

	return m
}

// Init implements tea.Model.
func (m Model) Init() tea.Cmd {
	cmds := []tea.Cmd{textinput.Blink}
	if m.search.Value() != "" {
		cmds = append(cmds, m.doFetch(m.search.Value()), m.spin.Tick)
	}
	return tea.Batch(cmds...)
}

// doFetch returns a tea.Cmd that fetches the given word.
func (m Model) doFetch(word string) tea.Cmd {
	return func() tea.Msg {
		entry, err := m.client.Fetch(word)
		if err != nil {
			if nf, ok := err.(*api.NotFoundError); ok {
				return NotFoundMsg{Word: word, Suggestions: nf.Suggestions}
			}
			return FetchErrMsg{Word: word, Err: err}
		}
		return WordFetchedMsg{Word: word, Entry: entry}
	}
}

// Update implements tea.Model.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m = m.resize()

	case spinner.TickMsg:
		if m.loading {
			var cmd tea.Cmd
			m.spin, cmd = m.spin.Update(msg)
			cmds = append(cmds, cmd)
		}

	case WordFetchedMsg:
		m.loading = false
		m.err = ""
		m.currentWord = msg.Word
		m.cache[msg.Word] = msg.Entry
		m.entry = msg.Entry
		m.store.AddHistory(msg.Word)
		m.search.SetSuggestions(m.store.History())
		m.content.SetContent(ui.RenderEntry(msg.Entry, m.content.Width))
		m.content.GotoTop()
		m.search.Focus()

	case NotFoundMsg:
		m.loading = false
		if len(msg.Suggestions) > 0 {
			tips := msg.Suggestions
			if len(tips) > 3 {
				tips = tips[:3]
			}
			m.err = fmt.Sprintf("No results for %q. Did you mean: %s?",
				msg.Word, strings.Join(tips, ", "))
		} else {
			m.err = fmt.Sprintf("No results for %q.", msg.Word)
		}
		m.content.SetContent(ui.RenderError(m.err))
		m.search.Focus()

	case FetchErrMsg:
		m.loading = false
		m.err = fmt.Sprintf("Error looking up %q: %v", msg.Word, msg.Err)
		m.content.SetContent(ui.RenderError(m.err))
		m.search.Focus()

	case tea.MouseMsg:
		var cmd tea.Cmd
		m.content, cmd = m.content.Update(msg)
		cmds = append(cmds, cmd)

	case tea.KeyMsg:
		switch {
		case msg.Type == tea.KeyEsc || msg.String() == "ctrl+c":
			return m, tea.Quit
		case msg.Type == tea.KeyEnter:
			word := strings.TrimSpace(m.search.Value())
			if word != "" {
				m.loading = true
				m.content.SetContent(fmt.Sprintf("Looking up %q…", word))
				cmds = append(cmds, m.doFetch(word), m.spin.Tick)
			}
		default:
			var tiCmd tea.Cmd
			m.search, tiCmd = m.search.Update(msg)
			cmds = append(cmds, tiCmd)
		}
	}

	return m, tea.Batch(cmds...)
}

// resize recalculates component dimensions after a WindowSizeMsg.
func (m Model) resize() Model {
	innerW := m.width - 2

	searchH := 3 // 1 input row + 2 border rows
	contentInnerH := m.height - searchH - 2
	if contentInnerH < 1 {
		contentInnerH = 1
	}

	m.search.Width = innerW - 2
	m.content.Width = innerW
	m.content.Height = contentInnerH

	if m.entry != nil {
		m.content.SetContent(ui.RenderEntry(m.entry, innerW))
	}

	return m
}

// View implements tea.Model.
func (m Model) View() string {
	if m.width == 0 {
		return ""
	}

	innerW := m.width - 2

	searchBox := ui.BorderInactive().Width(innerW).Render(m.search.View())

	var inner string
	if m.loading {
		inner = m.spin.View() + " Looking up…"
	} else {
		inner = m.content.View()
	}
	contentBox := ui.BorderInactive().Width(innerW).Render(inner)

	return lipgloss.JoinVertical(lipgloss.Left, searchBox, contentBox)
}
```

- [ ] **Step 2: Verify compile (non-test)**

```bash
go build ./internal/app/... 2>&1 | grep -v '_test.go'
```

Expected: no output. Test file errors are fine — fixed in Task 5.

- [ ] **Step 3: Commit**

```bash
git add internal/app/model.go
git commit -m "feat(minimal): rewrite model — search+results only, always-on insert"
```

---

### Task 4: Update cmd/root.go

**Files:**
- Modify: `cmd/root.go`

- [ ] **Step 1: Replace cmd/root.go**

```go
package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"

	"github.com/peterqlin/lazydict/config"
	"github.com/peterqlin/lazydict/internal/app"
	"github.com/peterqlin/lazydict/internal/store"
)

var rootCmd = &cobra.Command{
	Use:   "lazydict [word]",
	Short: "A terminal UI for the Merriam-Webster dictionary",
	Args:  cobra.MaximumNArgs(1),
	RunE:  run,
}

// Execute is the entrypoint called from main.go.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

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

	initialWord := ""
	if len(args) > 0 {
		initialWord = args[0]
	}

	m := app.New(cfg, st, initialWord)
	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion())
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("TUI error: %w", err)
	}
	return nil
}
```

- [ ] **Step 2: Verify full build**

```bash
go build ./... 2>&1 | grep -v '_test.go'
```

Expected: no output.

- [ ] **Step 3: Commit**

```bash
git add cmd/root.go
git commit -m "feat(minimal): drop flagStore from startup, enable mouse scroll"
```

---

### Task 5: Replace model_test.go

**Files:**
- Modify: `internal/app/model_test.go`

All old tests cover removed features (sections, panes, flags, favorites). Replace with tests for the new behavior.

- [ ] **Step 1: Replace model_test.go**

```go
package app_test

import (
	"fmt"
	"path/filepath"
	"testing"
	"time"

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
	return app.New(cfg, st, "")
}

func TestSearchAlwaysFocused(t *testing.T) {
	m := newTestModel(t)
	if !m.SearchFocused() {
		t.Error("expected search to be focused on launch")
	}
}

func TestEscQuits(t *testing.T) {
	m := newTestModel(t)
	tm := teatest.NewTestModel(t, m, teatest.WithInitialTermSize(120, 40))
	tm.Send(tea.KeyMsg{Type: tea.KeyEsc})
	tm.WaitFinished(t, teatest.WithFinalTimeout(3*time.Second))
}

func TestCtrlCQuits(t *testing.T) {
	m := newTestModel(t)
	tm := teatest.NewTestModel(t, m, teatest.WithInitialTermSize(120, 40))
	tm.Send(tea.KeyMsg{Type: tea.KeyCtrlC})
	tm.WaitFinished(t, teatest.WithFinalTimeout(3*time.Second))
}

func TestEnterWithEmptyInputNoFetch(t *testing.T) {
	m := newTestModel(t)
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m2.(app.Model).Loading() {
		t.Error("expected no fetch when submitting empty search")
	}
}

func TestWordFetchedSetsCurrentWord(t *testing.T) {
	m := newTestModel(t)
	entry := &api.Entry{}
	m2, _ := m.Update(app.WordFetchedMsg{Word: "serendipity", Entry: entry})
	if m2.(app.Model).CurrentWord() != "serendipity" {
		t.Errorf("expected currentWord=serendipity, got %q", m2.(app.Model).CurrentWord())
	}
}

func TestWordFetchedWritesHistory(t *testing.T) {
	cfg := &config.Config{MWKey: "test", MWThesKey: "test"}
	st, _ := store.New(filepath.Join(t.TempDir(), "data.json"))
	m := app.New(cfg, st, "")
	entry := &api.Entry{}
	m.Update(app.WordFetchedMsg{Word: "ephemeral", Entry: entry})
	hist := st.History()
	if len(hist) == 0 || hist[0] != "ephemeral" {
		t.Errorf("expected ephemeral at top of history, got %v", hist)
	}
}

func TestNotFoundSetsErr(t *testing.T) {
	m := newTestModel(t)
	m2, _ := m.Update(app.NotFoundMsg{Word: "xyzzy", Suggestions: nil})
	if m2.(app.Model).Err() == "" {
		t.Error("expected non-empty error after NotFoundMsg")
	}
}

func TestFetchErrSetsErr(t *testing.T) {
	m := newTestModel(t)
	m2, _ := m.Update(app.FetchErrMsg{Word: "xyzzy", Err: fmt.Errorf("timeout")})
	if m2.(app.Model).Err() == "" {
		t.Error("expected non-empty error after FetchErrMsg")
	}
}

func TestWindowSizeDoesNotPanic(t *testing.T) {
	m := newTestModel(t)
	m2, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	_ = m2.(app.Model).View()
}
```

- [ ] **Step 2: Run tests**

```bash
go test ./internal/app/... -v
```

Expected: all tests pass. If `TestCtrlCQuits` hangs, verify `tea.KeyCtrlC` sends `ctrl+c` in your bubbletea version — fallback: `tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{3}}` (ASCII ETX).

- [ ] **Step 3: Commit**

```bash
git add internal/app/model_test.go
git commit -m "test(minimal): replace tests for minimal model behavior"
```

---

### Task 6: Smoke test

**Files:** none

- [ ] **Step 1: Build**

```bash
go build -o lazydict_minimal .
```

Expected: exits 0, produces `lazydict_minimal` binary.

- [ ] **Step 2: Run and verify layout**

```bash
./lazydict_minimal
```

Verify:
- Two boxes stacked vertically, full width, rounded borders, no labels
- Cursor is in the search box immediately — no need to press `i`
- Typing goes into the search box
- `enter` on a non-empty word shows spinner then result
- Mouse wheel scrolls the results box up/down
- `esc` quits immediately

- [ ] **Step 3: Verify autofill**

Look up one word, then start typing it again — autocomplete suggestion should appear in the search box.

- [ ] **Step 4: Verify initial word arg**

```bash
./lazydict_minimal ephemeral
```

Expected: launches and immediately fetches "ephemeral".

- [ ] **Step 5: Clean up binary**

```bash
rm lazydict_minimal
```

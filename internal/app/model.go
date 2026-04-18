package app

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/peterqlin/lazydict/config"
	"github.com/peterqlin/lazydict/internal/api"
	"github.com/peterqlin/lazydict/internal/store"
	"github.com/peterqlin/lazydict/internal/ui"
)

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

// Model is the root BubbleTea model.
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
	flagSnap  viewport.Model
	flagNote  textarea.Model
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

// Exported accessors for tests.
func (m Model) TypingMode() bool       { return m.insertTarget != insertNone }
func (m Model) FocusedPane() pane      { return m.focused }
func (m Model) ActiveSection() section { return m.activeSection }

// Exported constants for tests.
const (
	PaneLeft  = paneLeft
	PaneRight = paneRight

	SectionSearch    = sectionSearch
	SectionHistory   = sectionHistory
	SectionFavorites = sectionFavorites
	SectionFlags     = sectionFlags
)

// New creates a new Model.
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
		focused:       paneLeft,
		activeSection: sectionSearch,
		insertTarget:  insertSearch,
		search:        ti,
		spin:          sp,
		cache:         make(map[string]*api.Entry),
		store:         st,
		flagStore:     fs,
		cfg:           cfg,
		client:        api.NewClient(cfg.MWKey, cfg.MWThesKey),
		keys:          DefaultKeyMap(),
	}

	m.history = ui.NewWordList(st.History(), 0, 0)
	m.favorites = ui.NewWordList(st.Favorites(), 0, 0)
	m.flags = ui.NewWordList(fs.Words(), 0, 0)
	m.search.SetSuggestions(st.History())
	m.content = viewport.New(0, 0)
	m.content.SetContent(ui.RenderWelcome())
	m.flagSnap = viewport.New(0, 0)

	ta := textarea.New()
	ta.Placeholder = "Describe the issue…"
	ta.CharLimit = 500
	ta.ShowLineNumbers = false
	m.flagNote = ta

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
		ui.SetWords(&m.history, m.store.History())
		m.search.SetSuggestions(m.store.History())
		m.content.SetContent(ui.RenderEntry(msg.Entry, m.content.Width))
		m.content.GotoTop()

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

	case FetchErrMsg:
		m.loading = false
		m.err = fmt.Sprintf("Error looking up %q: %v", msg.Word, msg.Err)
		m.content.SetContent(ui.RenderError(m.err))

	case tea.KeyMsg:
		wasInSearch := m.insertTarget == insertSearch
		wasInFlagNote := m.insertTarget == insertFlagNote
		cmd := m.handleKey(msg)
		cmds = append(cmds, cmd)
		// Forward to textinput only if we were already in search insert mode before
		// this key — prevents the activating key (e.g. "i") from being echoed.
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
	}

	return m, tea.Batch(cmds...)
}

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
			m.flagNote.Blur()
			if word := ui.SelectedWord(m.flags); word != "" {
				m.flagStore.UpdateNote(word, m.flagNote.Value())
			}
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
			m.insertTarget = insertFlagNote
			m.flagNote.Focus()
			return textarea.Blink
		}

	case key.Matches(msg, m.keys.ClearSearch) && m.activeSection == sectionSearch:
		m.search.SetValue("")
		m.focused = paneLeft
		m.insertTarget = insertSearch
		m.search.Focus()
		return textinput.Blink

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

	case key.Matches(msg, m.keys.Section1) && m.focused == paneLeft:
		m.activeSection = sectionSearch

	case key.Matches(msg, m.keys.Section2) && m.focused == paneLeft:
		m.activeSection = sectionHistory

	case key.Matches(msg, m.keys.Section3) && m.focused == paneLeft:
		m.activeSection = sectionFavorites

	case key.Matches(msg, m.keys.Section4) && m.focused == paneLeft:
		m.activeSection = sectionFlags
		m.loadFlagDetail()

	case key.Matches(msg, m.keys.Up) && m.focused == paneLeft:
		switch m.activeSection {
		case sectionHistory:
			m.history.CursorUp()
		case sectionFavorites:
			m.favorites.CursorUp()
		case sectionFlags:
			m.flags.CursorUp()
			m.loadFlagDetail()
		}

	case key.Matches(msg, m.keys.Down) && m.focused == paneLeft:
		switch m.activeSection {
		case sectionHistory:
			m.history.CursorDown()
		case sectionFavorites:
			m.favorites.CursorDown()
		case sectionFlags:
			m.flags.CursorDown()
			m.loadFlagDetail()
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
				hist := m.store.History()
				ui.SetWords(&m.history, hist)
				m.search.SetSuggestions(hist)
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
			if word := ui.SelectedWord(m.flags); word != "" {
				m.flagStore.Delete(word)
				ui.SetWords(&m.flags, m.flagStore.Words())
				m.loadFlagDetail()
			}
		}

	case key.Matches(msg, m.keys.Flag) && m.currentWord != "":
		m.flagStore.Add(m.currentWord, m.content.View())
		ui.SetWords(&m.flags, m.flagStore.Words())
	}

	return nil
}

// resize recalculates component dimensions after a WindowSizeMsg.
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

	snapH := rightInnerH * 6 / 10
	noteH := rightInnerH - snapH - 4 // subtract borders for two panels
	noteH = max(noteH, 2)
	m.flagSnap.Width = rightInnerW
	m.flagSnap.Height = snapH
	m.flagNote.SetWidth(rightInnerW)
	m.flagNote.SetHeight(noteH)

	if m.entry != nil {
		m.content.SetContent(ui.RenderEntry(m.entry, rightInnerW))
	}

	return m
}

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

// View implements tea.Model.
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

func (m Model) renderSearch(width int) string {
	active := m.focused == paneLeft && m.activeSection == sectionSearch
	return ui.BorderWithTitle(m.search.View(), "Search", 1, width, active)
}

func (m Model) renderWordList(l list.Model, label string, sec section, width int) string {
	active := m.focused == paneLeft && m.activeSection == sec
	return ui.BorderWithTitle(l.View(), label, int(sec)+1, width, active)
}

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
	snapBorder := ui.BorderWithTitle(m.flagSnap.View(), "Definition Snapshot", 0, rightW, snapActive)

	noteActive := m.insertTarget == insertFlagNote
	noteBorder := ui.BorderWithTitle(m.flagNote.View(), "Note", 0, rightW, noteActive)

	return lipgloss.JoinVertical(lipgloss.Left, snapBorder, noteBorder)
}

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

func (m Model) renderStatusBar() string {
	var parts []string
	if m.insertTarget != insertNone {
		parts = []string{
			ui.KeyHint("esc") + " cancel",
			ui.KeyHint("enter") + " search",
		}
	} else {
		parts = []string{
			ui.KeyHint("i") + " search",
			ui.KeyHint("j/k") + " navigate",
			ui.KeyHint("1-4") + " section",
			ui.KeyHint("tab") + " switch pane",
			ui.KeyHint("Shift-j/k") + " scroll",
			ui.KeyHint("b") + " bookmark",
			ui.KeyHint("d") + " delete",
			ui.KeyHint("q") + " quit",
		}
	}
	return ui.StatusBarStyle().Render(strings.Join(parts, "  "))
}

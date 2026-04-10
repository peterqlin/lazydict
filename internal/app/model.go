package app

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/230pe/lazydict/config"
	"github.com/230pe/lazydict/internal/api"
	"github.com/230pe/lazydict/internal/store"
	"github.com/230pe/lazydict/internal/ui"
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
)

// Model is the root BubbleTea model.
type Model struct {
	width  int
	height int

	focused       pane
	activeSection section
	typingMode    bool

	search    textinput.Model
	history   list.Model
	favorites list.Model
	content   viewport.Model
	spin      spinner.Model

	entry       *api.Entry
	cache       map[string]*api.Entry
	store       *store.Store
	cfg         *config.Config
	client      *api.Client

	loading     bool
	err         string
	currentWord string

	keys KeyMap
}

// Exported accessors for tests.
func (m Model) TypingMode() bool       { return m.typingMode }
func (m Model) FocusedPane() pane      { return m.focused }
func (m Model) ActiveSection() section { return m.activeSection }

// Exported constants for tests.
const (
	PaneLeft  = paneLeft
	PaneRight = paneRight

	SectionSearch    = sectionSearch
	SectionHistory   = sectionHistory
	SectionFavorites = sectionFavorites
)

// New creates a new Model.
func New(cfg *config.Config, st *store.Store, initialWord string) Model {
	ti := textinput.New()
	ti.Placeholder = "search…"
	ti.CharLimit = 100
	ti.Focus()

	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(ui.ColorAccent)

	m := Model{
		focused:       paneLeft,
		activeSection: sectionSearch,
		typingMode:    true,
		search:        ti,
		spin:          sp,
		cache:         make(map[string]*api.Entry),
		store:         st,
		cfg:           cfg,
		client:        api.NewClient(cfg.MWKey, cfg.MWThesKey),
		keys:          DefaultKeyMap(),
	}

	m.history = ui.NewWordList(st.History(), 0, 0)
	m.favorites = ui.NewWordList(st.Favorites(), 0, 0)
	m.content = viewport.New(0, 0)
	m.content.SetContent(ui.RenderWelcome())

	if initialWord != "" {
		m.search.SetValue(initialWord)
	}

	return m
}

// Init implements tea.Model.
func (m Model) Init() tea.Cmd {
	cmds := []tea.Cmd{textinput.Blink, m.spin.Tick}
	if m.search.Value() != "" {
		cmds = append(cmds, m.doFetch(m.search.Value()))
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
		var cmd tea.Cmd
		m.spin, cmd = m.spin.Update(msg)
		cmds = append(cmds, cmd)

	case WordFetchedMsg:
		m.loading = false
		m.err = ""
		m.currentWord = msg.Word
		m.cache[msg.Word] = msg.Entry
		m.entry = msg.Entry
		m.store.AddHistory(msg.Word)
		ui.SetWords(&m.history, m.store.History())
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
		cmd := m.handleKey(msg)
		cmds = append(cmds, cmd)
	}

	// Forward to textinput when in typing mode (handles cursor blink etc.)
	if m.typingMode {
		var cmd tea.Cmd
		m.search, cmd = m.search.Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

func (m *Model) handleKey(msg tea.KeyMsg) tea.Cmd {
	if m.typingMode {
		switch {
		case key.Matches(msg, m.keys.ExitTyping):
			m.typingMode = false
			m.search.Blur()
		case key.Matches(msg, m.keys.Submit):
			word := strings.TrimSpace(m.search.Value())
			if word != "" {
				m.typingMode = false
				m.search.Blur()
				m.loading = true
				m.content.SetContent(fmt.Sprintf("Looking up %q…", word))
				return tea.Batch(m.doFetch(word), m.spin.Tick)
			}
		}
		return nil
	}

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
		m.focused = paneLeft
		m.activeSection = sectionSearch
		m.typingMode = true
		m.search.Focus()
		return textinput.Blink

	case key.Matches(msg, m.keys.SectionLeft) && m.focused == paneLeft:
		m.activeSection = (m.activeSection + 2) % 3

	case key.Matches(msg, m.keys.SectionRight) && m.focused == paneLeft:
		m.activeSection = (m.activeSection + 1) % 3

	case key.Matches(msg, m.keys.Up) && m.focused == paneLeft:
		switch m.activeSection {
		case sectionHistory:
			m.history.CursorUp()
		case sectionFavorites:
			m.favorites.CursorUp()
		}

	case key.Matches(msg, m.keys.Down) && m.focused == paneLeft:
		switch m.activeSection {
		case sectionHistory:
			m.history.CursorDown()
		case sectionFavorites:
			m.favorites.CursorDown()
		}

	case key.Matches(msg, m.keys.Submit) && m.focused == paneLeft:
		var word string
		switch m.activeSection {
		case sectionHistory:
			word = ui.SelectedWord(m.history)
		case sectionFavorites:
			word = ui.SelectedWord(m.favorites)
		}
		if word != "" {
			if cached, ok := m.cache[word]; ok {
				m.entry = cached
				m.currentWord = word
				m.content.SetContent(ui.RenderEntry(cached, m.content.Width))
				m.content.GotoTop()
			} else {
				m.loading = true
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
		}
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
	historyH := remaining * 6 / 10
	favoritesH := remaining - historyH

	listInnerW := leftW - 2
	historyInnerH := historyH - 2
	favoritesInnerH := favoritesH - 2
	rightInnerW := rightW - 2
	rightInnerH := innerH - 2

	m.search.Width = listInnerW - 2
	m.history.SetSize(listInnerW, historyInnerH)
	m.favorites.SetSize(listInnerW, favoritesInnerH)
	m.content.Width = rightInnerW
	m.content.Height = rightInnerH

	if m.entry != nil {
		m.content.SetContent(ui.RenderEntry(m.entry, rightInnerW))
	}

	return m
}

// View implements tea.Model.
func (m Model) View() string {
	if m.width == 0 {
		return ""
	}

	leftW := ui.LeftPanelWidth(m.width)

	searchView := m.renderSearch(leftW)
	historyView := m.renderWordList(m.history, "HISTORY", sectionHistory, leftW)
	favoritesView := m.renderWordList(m.favorites, "FAVORITES", sectionFavorites, leftW)
	leftPane := lipgloss.JoinVertical(lipgloss.Left, searchView, historyView, favoritesView)

	rightPane := m.renderContent()

	main := lipgloss.JoinHorizontal(lipgloss.Top, leftPane, " ", rightPane)
	status := m.renderStatusBar()

	return lipgloss.JoinVertical(lipgloss.Left, main, status)
}

func (m Model) renderSearch(width int) string {
	active := m.focused == paneLeft && m.activeSection == sectionSearch
	title := ui.SectionTitle(active).Render("SEARCH")
	body := lipgloss.JoinVertical(lipgloss.Left, title, m.search.View())

	style := ui.BorderInactive().Width(width - 2)
	if active {
		style = ui.BorderActive().Width(width - 2)
	}
	return style.Render(body)
}

func (m Model) renderWordList(l list.Model, label string, sec section, width int) string {
	active := m.focused == paneLeft && m.activeSection == sec
	title := ui.SectionTitle(active).Render(label)
	body := lipgloss.JoinVertical(lipgloss.Left, title, l.View())

	style := ui.BorderInactive().Width(width - 2)
	if active {
		style = ui.BorderActive().Width(width - 2)
	}
	return style.Render(body)
}

func (m Model) renderContent() string {
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
	if m.typingMode {
		parts = []string{
			ui.KeyHint("esc") + " cancel",
			ui.KeyHint("enter") + " search",
		}
	} else {
		parts = []string{
			ui.KeyHint("i") + " search",
			ui.KeyHint("j/k") + " navigate",
			ui.KeyHint("h/l") + " section",
			ui.KeyHint("tab") + " switch pane",
			ui.KeyHint("Shift-j/k") + " scroll",
			ui.KeyHint("b") + " bookmark",
			ui.KeyHint("d") + " delete",
			ui.KeyHint("q") + " quit",
		}
	}
	return ui.StatusBarStyle(m.width).Render(strings.Join(parts, "  "))
}

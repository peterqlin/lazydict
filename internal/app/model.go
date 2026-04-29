package app

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
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
	keys        KeyMap

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
		keys:   DefaultKeyMap(),
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
		case key.Matches(msg, m.keys.Quit):
			return m, tea.Quit
		case key.Matches(msg, m.keys.Submit):
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
	innerW := max(m.width-2, 1)

	searchH := 3 // 1 input row + 2 border rows
	contentInnerH := max(m.height-searchH-2, 1)

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
	contentBox := ui.BorderInactive().Width(innerW).Height(m.content.Height).Render(inner)

	return lipgloss.JoinVertical(lipgloss.Left, searchBox, contentBox)
}

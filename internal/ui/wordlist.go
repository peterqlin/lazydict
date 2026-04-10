package ui

import (
	"fmt"
	"io"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type WordItem struct {
	Word string
}

func (w WordItem) FilterValue() string { return w.Word }
func (w WordItem) Title() string       { return w.Word }
func (w WordItem) Description() string { return "" }

type WordDelegate struct{}

func (d WordDelegate) Height() int                              { return 1 }
func (d WordDelegate) Spacing() int                            { return 0 }
func (d WordDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }
func (d WordDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	wi, ok := item.(WordItem)
	if !ok {
		return
	}
	if index == m.Index() {
		cursor := lipgloss.NewStyle().Foreground(ColorAccent).Render("▶")
		word := lipgloss.NewStyle().Foreground(ColorAccent).Render(wi.Word)
		fmt.Fprintf(w, "%s %s", cursor, word)
	} else {
		word := lipgloss.NewStyle().Foreground(ColorText).PaddingLeft(2).Render(wi.Word)
		fmt.Fprint(w, word)
	}
}

func NewWordList(words []string, width, height int) list.Model {
	items := wordsToItems(words)
	l := list.New(items, WordDelegate{}, width, height)
	l.SetShowTitle(false)
	l.SetShowHelp(false)
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)
	l.DisableQuitKeybindings()
	return l
}

func wordsToItems(words []string) []list.Item {
	items := make([]list.Item, len(words))
	for i, w := range words {
		items[i] = WordItem{Word: w}
	}
	return items
}

func SetWords(l *list.Model, words []string) {
	l.SetItems(wordsToItems(words))
}

func SelectedWord(l list.Model) string {
	if item := l.SelectedItem(); item != nil {
		if wi, ok := item.(WordItem); ok {
			return wi.Word
		}
	}
	return ""
}

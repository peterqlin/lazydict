package app

import "github.com/charmbracelet/bubbles/key"

type KeyMap struct {
	Quit         key.Binding
	SwitchPane   key.Binding
	SectionLeft  key.Binding
	SectionRight key.Binding
	Up           key.Binding
	Down         key.Binding
	EnterTyping  key.Binding
	ExitTyping   key.Binding
	Submit       key.Binding
	ScrollUp     key.Binding
	ScrollDown   key.Binding
	Bookmark     key.Binding
	Delete       key.Binding
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
			key.WithHelp("i", "search"),
		),
		ExitTyping: key.NewBinding(
			key.WithKeys("esc"),
			key.WithHelp("esc", "cancel"),
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
	}
}

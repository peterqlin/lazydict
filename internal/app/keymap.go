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

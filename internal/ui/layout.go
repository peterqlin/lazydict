package ui

import "github.com/charmbracelet/lipgloss"

var (
	ColorBlue    = lipgloss.Color("#58a6ff")
	ColorGold    = lipgloss.Color("#d2a679")
	ColorMuted   = lipgloss.Color("#6e7681")
	ColorDim     = lipgloss.Color("#444d56")
	ColorText    = lipgloss.Color("#cdd9e5")
	ColorAccent  = lipgloss.Color("#79c0ff")
	ColorBg      = lipgloss.Color("#0d1117")
	ColorBgPanel = lipgloss.Color("#161b22")
	ColorBorder  = lipgloss.Color("#30363d")
)

func BorderActive() lipgloss.Style {
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorBlue)
}

func BorderInactive() lipgloss.Style {
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorBorder)
}

func SectionTitle(active bool) lipgloss.Style {
	c := ColorMuted
	if active {
		c = ColorBlue
	}
	return lipgloss.NewStyle().Foreground(c).Bold(true)
}

func StatusBarStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(ColorMuted).
		PaddingLeft(1).
		PaddingRight(1)
}

func KeyHint(key string) string {
	return lipgloss.NewStyle().Foreground(ColorAccent).Render(key)
}

func LeftPanelWidth(total int) int {
	w := total / 4
	if w < 18 {
		w = 18
	}
	return w
}

func RightPanelWidth(total, leftWidth int) int {
	return total - leftWidth - 2
}

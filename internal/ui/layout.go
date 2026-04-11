package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	ColorBlue    = lipgloss.Color("#58a6ff")
	ColorGold    = lipgloss.Color("#d2a679")
	ColorMuted   = lipgloss.Color("#6e7681")
	ColorDim     = lipgloss.Color("#444d56")
	ColorText    = lipgloss.Color("#cdd9e5")
	ColorAccent  = lipgloss.Color("#79c0ff")
	ColorBg      = lipgloss.Color("#0d1117")
	ColorBgPanel = lipgloss.Color("#161b22")
	ColorBorder  = lipgloss.Color("#d4dce0")
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


func StatusBarStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(ColorMuted).
		PaddingLeft(1).
		PaddingRight(1)
}

// BorderWithTitle renders a bordered panel with the section title embedded in the top
// border line, e.g. ╭─[1]─Search──────────╮. width is the total outer width.
func BorderWithTitle(content, label string, num, width int, active bool) string {
	borderColor := ColorBorder
	if active {
		borderColor = ColorBlue
	}

	bc := lipgloss.NewStyle().Foreground(borderColor)
	numStyle := lipgloss.NewStyle().Foreground(ColorAccent)
	lblStyle := lipgloss.NewStyle().Foreground(ColorBorder)
	if active {
		lblStyle = lipgloss.NewStyle().Foreground(ColorBlue).Bold(true)
	}

	numStr := fmt.Sprintf("[%d]", num)
	// All chars are ASCII so len() == visual width.
	titleSeg := bc.Render("─") + numStyle.Render(numStr) + bc.Render("─") + lblStyle.Render(label)
	titleSegW := 1 + len(numStr) + 1 + len(label)

	innerW := width - 2 // minus two corner chars
	fillW := innerW - titleSegW
	if fillW < 0 {
		fillW = 0
	}
	topLine := bc.Render("╭") + titleSeg + bc.Render(strings.Repeat("─", fillW)) + bc.Render("╮")

	body := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderTop(false).
		BorderForeground(borderColor).
		Width(width - 2).
		Render(content)

	return topLine + "\n" + body
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

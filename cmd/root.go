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

	dataPath := filepath.Join(filepath.Dir(cfgPath), "data.json")
	if err := os.MkdirAll(filepath.Dir(dataPath), 0700); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}

	st, err := store.New(dataPath)
	if err != nil {
		return fmt.Errorf("open store: %w", err)
	}

	initialWord := ""
	if len(args) > 0 {
		initialWord = args[0]
	}

	m := app.New(cfg, st, initialWord)
	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("TUI error: %w", err)
	}
	return nil
}

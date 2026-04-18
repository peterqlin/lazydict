package app_test

import (
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

func TestLaunchesInTypingMode(t *testing.T) {
	m := newTestModel(t)
	if !m.TypingMode() {
		t.Error("expected typing mode on launch")
	}
}

func TestEscExitsTypingMode(t *testing.T) {
	m := newTestModel(t)
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m2 := updated.(app.Model)
	if m2.TypingMode() {
		t.Error("expected typing mode to be off after Esc")
	}
}

func TestIEntersTypingMode(t *testing.T) {
	m := newTestModel(t)
	// Exit typing mode first
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m2 := updated.(app.Model)
	// Press i to re-enter
	updated2, _ := m2.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'i'}})
	m3 := updated2.(app.Model)
	if !m3.TypingMode() {
		t.Error("expected typing mode after pressing i")
	}
}

func TestQuitInNavMode(t *testing.T) {
	m := newTestModel(t)
	// Exit typing mode
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m2 := updated.(app.Model)

	tm := teatest.NewTestModel(t, m2, teatest.WithInitialTermSize(120, 40))
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	tm.WaitFinished(t, teatest.WithFinalTimeout(3*time.Second))
}

func TestTabSwitchesPane(t *testing.T) {
	m := newTestModel(t)
	// Exit typing mode first
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m2 := updated.(app.Model)
	if m2.FocusedPane() != app.PaneLeft {
		t.Fatal("expected left pane focus initially")
	}
	updated2, _ := m2.Update(tea.KeyMsg{Type: tea.KeyTab})
	m3 := updated2.(app.Model)
	if m3.FocusedPane() != app.PaneRight {
		t.Error("expected right pane focus after Tab")
	}
}

func TestHLCyclesSections(t *testing.T) {
	m := newTestModel(t)
	// Exit typing mode — start in sectionSearch
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m2 := updated.(app.Model)

	// l → sectionHistory
	updated2, _ := m2.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}})
	m3 := updated2.(app.Model)
	if m3.ActiveSection() != app.SectionHistory {
		t.Errorf("expected History after l, got %v", m3.ActiveSection())
	}

	// l → sectionFavorites
	updated3, _ := m3.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}})
	m4 := updated3.(app.Model)
	if m4.ActiveSection() != app.SectionFavorites {
		t.Errorf("expected Favorites after l, got %v", m4.ActiveSection())
	}

	// h → back to sectionHistory
	updated4, _ := m4.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h'}})
	m5 := updated4.(app.Model)
	if m5.ActiveSection() != app.SectionHistory {
		t.Errorf("expected History after h, got %v", m5.ActiveSection())
	}
}

func TestCacheHitMovesToHistoryTop(t *testing.T) {
	cfg := &config.Config{MWKey: "test", MWThesKey: "test"}
	st, _ := store.New(filepath.Join(t.TempDir(), "data.json"))
	m := app.New(cfg, st, "")
	entry := &api.Entry{}
	// Load alpha then beta — history becomes ["beta", "alpha"]
	m2, _ := m.Update(app.WordFetchedMsg{Word: "alpha", Entry: entry})
	m3, _ := m2.(app.Model).Update(app.WordFetchedMsg{Word: "beta", Entry: entry})
	// Exit typing mode, go to History section, cursor down to "alpha", press enter
	m4, _ := m3.(app.Model).Update(tea.KeyMsg{Type: tea.KeyEsc})
	m5, _ := m4.(app.Model).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'2'}})
	m6, _ := m5.(app.Model).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	_, _ = m6.(app.Model).Update(tea.KeyMsg{Type: tea.KeyEnter})
	// History should now be: ["alpha", "beta"]
	hist := st.History()
	if hist[0] != "alpha" {
		t.Errorf("expected alpha at top after cache hit, got %q", hist[0])
	}
}

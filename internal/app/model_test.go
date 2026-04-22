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
	fs, err := store.NewFlagStore(filepath.Join(t.TempDir(), "flags.json"))
	if err != nil {
		t.Fatalf("flagstore: %v", err)
	}
	return app.New(cfg, st, fs, "")
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

func TestIEntersTypingModeWhenSearchActive(t *testing.T) {
	m := newTestModel(t)
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m2 := updated.(app.Model)
	updated2, _ := m2.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'i'}})
	m3 := updated2.(app.Model)
	if !m3.TypingMode() {
		t.Error("expected typing mode after pressing i with search active")
	}
}

func TestIIsNoopWhenHistoryActive(t *testing.T) {
	m := newTestModel(t)
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m2 := updated.(app.Model)
	updated2, _ := m2.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}})
	m3 := updated2.(app.Model)
	updated3, _ := m3.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'i'}})
	m4 := updated3.(app.Model)
	if m4.TypingMode() {
		t.Error("expected i to be no-op when history section active")
	}
}

func TestClearSearchEntersTypingMode(t *testing.T) {
	m := newTestModel(t)
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m2 := updated.(app.Model)
	updated2, _ := m2.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	m3 := updated2.(app.Model)
	if !m3.TypingMode() {
		t.Error("expected c to enter typing mode in search section")
	}
}

func TestQuitInNavMode(t *testing.T) {
	m := newTestModel(t)
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m2 := updated.(app.Model)
	tm := teatest.NewTestModel(t, m2, teatest.WithInitialTermSize(120, 40))
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	tm.WaitFinished(t, teatest.WithFinalTimeout(3*time.Second))
}

func TestTabSwitchesPane(t *testing.T) {
	m := newTestModel(t)
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

func TestHLCyclesFourSections(t *testing.T) {
	m := newTestModel(t)
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m2 := updated.(app.Model)

	u, _ := m2.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}})
	if u.(app.Model).ActiveSection() != app.SectionHistory {
		t.Errorf("expected History after l from Search, got %v", u.(app.Model).ActiveSection())
	}
	u, _ = u.(app.Model).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}})
	if u.(app.Model).ActiveSection() != app.SectionFavorites {
		t.Errorf("expected Favorites after l, got %v", u.(app.Model).ActiveSection())
	}
	u, _ = u.(app.Model).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}})
	if u.(app.Model).ActiveSection() != app.SectionFlags {
		t.Errorf("expected Flags after l from Favorites, got %v", u.(app.Model).ActiveSection())
	}
	u, _ = u.(app.Model).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}})
	if u.(app.Model).ActiveSection() != app.SectionSearch {
		t.Errorf("expected Search after l from Flags (wrap), got %v", u.(app.Model).ActiveSection())
	}
	u, _ = u.(app.Model).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h'}})
	if u.(app.Model).ActiveSection() != app.SectionFlags {
		t.Errorf("expected Flags after h from Search (wrap), got %v", u.(app.Model).ActiveSection())
	}
}

func TestCacheHitMovesToHistoryTop(t *testing.T) {
	cfg := &config.Config{MWKey: "test", MWThesKey: "test"}
	st, _ := store.New(filepath.Join(t.TempDir(), "data.json"))
	fs, _ := store.NewFlagStore(filepath.Join(t.TempDir(), "flags.json"))
	m := app.New(cfg, st, fs, "")
	entry := &api.Entry{}
	m2, _ := m.Update(app.WordFetchedMsg{Word: "alpha", Entry: entry})
	m3, _ := m2.(app.Model).Update(app.WordFetchedMsg{Word: "beta", Entry: entry})
	m4, _ := m3.(app.Model).Update(tea.KeyMsg{Type: tea.KeyEsc})
	m5, _ := m4.(app.Model).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'2'}})
	m6, _ := m5.(app.Model).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	_, _ = m6.(app.Model).Update(tea.KeyMsg{Type: tea.KeyEnter})
	hist := st.History()
	if hist[0] != "alpha" {
		t.Errorf("expected alpha at top after cache hit, got %q", hist[0])
	}
}

func TestFlagCurrentWord(t *testing.T) {
	cfg := &config.Config{MWKey: "test", MWThesKey: "test"}
	st, _ := store.New(filepath.Join(t.TempDir(), "data.json"))
	fs, _ := store.NewFlagStore(filepath.Join(t.TempDir(), "flags.json"))
	m := app.New(cfg, st, fs, "")

	entry := &api.Entry{}
	m2, _ := m.Update(app.WordFetchedMsg{Word: "ephemeral", Entry: entry})
	m3, _ := m2.(app.Model).Update(tea.KeyMsg{Type: tea.KeyEsc})

	_, _ = m3.(app.Model).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'f'}})

	entries := fs.All()
	if len(entries) != 1 || entries[0].Word != "ephemeral" {
		t.Errorf("expected ephemeral to be flagged, got %+v", entries)
	}
}

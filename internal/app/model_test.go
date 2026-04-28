package app_test

import (
	"fmt"
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

func TestSearchAlwaysFocused(t *testing.T) {
	m := newTestModel(t)
	if !m.SearchFocused() {
		t.Error("expected search to be focused on launch")
	}
}

func TestEscQuits(t *testing.T) {
	m := newTestModel(t)
	tm := teatest.NewTestModel(t, m, teatest.WithInitialTermSize(120, 40))
	tm.Send(tea.KeyMsg{Type: tea.KeyEsc})
	tm.WaitFinished(t, teatest.WithFinalTimeout(3*time.Second))
}

func TestCtrlCQuits(t *testing.T) {
	m := newTestModel(t)
	tm := teatest.NewTestModel(t, m, teatest.WithInitialTermSize(120, 40))
	tm.Send(tea.KeyMsg{Type: tea.KeyCtrlC})
	tm.WaitFinished(t, teatest.WithFinalTimeout(3*time.Second))
}

func TestEnterWithEmptyInputNoFetch(t *testing.T) {
	m := newTestModel(t)
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m2.(app.Model).Loading() {
		t.Error("expected no fetch when submitting empty search")
	}
}

func TestWordFetchedSetsCurrentWord(t *testing.T) {
	m := newTestModel(t)
	entry := &api.Entry{}
	m2, _ := m.Update(app.WordFetchedMsg{Word: "serendipity", Entry: entry})
	if m2.(app.Model).CurrentWord() != "serendipity" {
		t.Errorf("expected currentWord=serendipity, got %q", m2.(app.Model).CurrentWord())
	}
}

func TestWordFetchedWritesHistory(t *testing.T) {
	cfg := &config.Config{MWKey: "test", MWThesKey: "test"}
	st, _ := store.New(filepath.Join(t.TempDir(), "data.json"))
	m := app.New(cfg, st, "")
	entry := &api.Entry{}
	m.Update(app.WordFetchedMsg{Word: "ephemeral", Entry: entry})
	hist := st.History()
	if len(hist) == 0 || hist[0] != "ephemeral" {
		t.Errorf("expected ephemeral at top of history, got %v", hist)
	}
}

func TestNotFoundSetsErr(t *testing.T) {
	m := newTestModel(t)
	m2, _ := m.Update(app.NotFoundMsg{Word: "xyzzy", Suggestions: nil})
	if m2.(app.Model).Err() == "" {
		t.Error("expected non-empty error after NotFoundMsg")
	}
}

func TestFetchErrSetsErr(t *testing.T) {
	m := newTestModel(t)
	m2, _ := m.Update(app.FetchErrMsg{Word: "xyzzy", Err: fmt.Errorf("timeout")})
	if m2.(app.Model).Err() == "" {
		t.Error("expected non-empty error after FetchErrMsg")
	}
}

func TestWindowSizeDoesNotPanic(t *testing.T) {
	m := newTestModel(t)
	m2, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	_ = m2.(app.Model).View()
}

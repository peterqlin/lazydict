package store_test

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/230pe/lazydict/internal/store"
)

func newStore(t *testing.T) *store.Store {
	t.Helper()
	s, err := store.New(filepath.Join(t.TempDir(), "data.json"))
	if err != nil {
		t.Fatalf("store.New: %v", err)
	}
	return s
}

func TestHistoryAddAndOrder(t *testing.T) {
	s := newStore(t)
	s.AddHistory("alpha")
	s.AddHistory("beta")
	s.AddHistory("gamma")
	h := s.History()
	if h[0] != "gamma" || h[1] != "beta" || h[2] != "alpha" {
		t.Errorf("unexpected order: %v", h)
	}
}

func TestHistoryDedup(t *testing.T) {
	s := newStore(t)
	s.AddHistory("alpha")
	s.AddHistory("beta")
	s.AddHistory("alpha") // should bubble to top
	h := s.History()
	if len(h) != 2 {
		t.Errorf("expected 2 entries, got %d: %v", len(h), h)
	}
	if h[0] != "alpha" {
		t.Errorf("expected alpha at top, got %q", h[0])
	}
}

func TestHistoryCap(t *testing.T) {
	s := newStore(t)
	for i := 0; i < 105; i++ {
		s.AddHistory(fmt.Sprintf("word%d", i))
	}
	if len(s.History()) != 100 {
		t.Errorf("expected 100 entries, got %d", len(s.History()))
	}
}

func TestHistoryRemove(t *testing.T) {
	s := newStore(t)
	s.AddHistory("alpha")
	s.AddHistory("beta")
	s.RemoveHistory("alpha")
	h := s.History()
	if len(h) != 1 || h[0] != "beta" {
		t.Errorf("unexpected history after remove: %v", h)
	}
}

func TestFavoriteToggle(t *testing.T) {
	s := newStore(t)
	s.ToggleFavorite("alpha")
	if !s.IsFavorite("alpha") {
		t.Error("expected alpha to be a favorite")
	}
	s.ToggleFavorite("alpha")
	if s.IsFavorite("alpha") {
		t.Error("expected alpha to be removed from favorites")
	}
}

func TestFavoriteRemove(t *testing.T) {
	s := newStore(t)
	s.ToggleFavorite("alpha")
	s.ToggleFavorite("beta")
	s.RemoveFavorite("alpha")
	favs := s.Favorites()
	if len(favs) != 1 || favs[0] != "beta" {
		t.Errorf("unexpected favorites after remove: %v", favs)
	}
}

func TestPersistence(t *testing.T) {
	path := filepath.Join(t.TempDir(), "data.json")
	s1, _ := store.New(path)
	s1.AddHistory("alpha")
	s1.ToggleFavorite("beta")

	s2, err := store.New(path)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if len(s2.History()) != 1 || s2.History()[0] != "alpha" {
		t.Errorf("history not persisted: %v", s2.History())
	}
	if !s2.IsFavorite("beta") {
		t.Error("favorites not persisted")
	}
}

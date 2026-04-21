package store

import (
	"encoding/json"
	"fmt"
	"os"
	"slices"
	"sync"
)

const maxHistory = 1000

type data struct {
	History   []string `json:"history"`
	Favorites []string `json:"favorites"`
}

// Store manages persistent history and favorites.
type Store struct {
	mu   sync.Mutex
	path string
	d    data
}

// New loads or creates the store at path.
func New(path string) (*Store, error) {
	s := &Store{path: path}
	b, err := os.ReadFile(path)
	if err == nil {
		_ = json.Unmarshal(b, &s.d)
	}
	if s.d.History == nil {
		s.d.History = []string{}
	}
	if s.d.Favorites == nil {
		s.d.Favorites = []string{}
	}
	return s, nil
}

func (s *Store) save() {
	b, err := json.MarshalIndent(s.d, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "lazydict: store marshal error: %v\n", err)
		return
	}
	if err := os.WriteFile(s.path, b, 0600); err != nil {
		fmt.Fprintf(os.Stderr, "lazydict: store write error: %v\n", err)
	}
}

// AddHistory prepends word, deduplicates, and caps at maxHistory.
func (s *Store) AddHistory(word string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.d.History = slices.DeleteFunc(s.d.History, func(w string) bool { return w == word })
	s.d.History = append([]string{word}, s.d.History...)
	if len(s.d.History) > maxHistory {
		s.d.History = s.d.History[:maxHistory]
	}
	s.save()
}

// RemoveHistory removes word from history.
func (s *Store) RemoveHistory(word string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.d.History = slices.DeleteFunc(s.d.History, func(w string) bool { return w == word })
	s.save()
}

// History returns all history entries (newest first).
func (s *Store) History() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]string, len(s.d.History))
	copy(out, s.d.History)
	return out
}

// ToggleFavorite adds or removes word from favorites.
func (s *Store) ToggleFavorite(word string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if slices.Contains(s.d.Favorites, word) {
		s.d.Favorites = slices.DeleteFunc(s.d.Favorites, func(w string) bool { return w == word })
	} else {
		s.d.Favorites = append(s.d.Favorites, word)
	}
	s.save()
}

// RemoveFavorite removes word from favorites unconditionally.
func (s *Store) RemoveFavorite(word string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.d.Favorites = slices.DeleteFunc(s.d.Favorites, func(w string) bool { return w == word })
	s.save()
}

// IsFavorite reports whether word is bookmarked.
func (s *Store) IsFavorite(word string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Contains(s.d.Favorites, word)
}

// Favorites returns all favorites (add-order preserved).
func (s *Store) Favorites() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]string, len(s.d.Favorites))
	copy(out, s.d.Favorites)
	return out
}

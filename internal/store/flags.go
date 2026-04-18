package store

import (
	"encoding/json"
	"fmt"
	"os"
	"slices"
	"sync"
	"time"
)

type FlagEntry struct {
	Word      string    `json:"word"`
	Snapshot  string    `json:"snapshot"`
	Note      string    `json:"note"`
	CreatedAt time.Time `json:"created_at"`
}

type FlagStore struct {
	mu      sync.Mutex
	path    string
	entries []FlagEntry
}

func NewFlagStore(path string) (*FlagStore, error) {
	fs := &FlagStore{path: path}
	b, err := os.ReadFile(path)
	if err == nil {
		_ = json.Unmarshal(b, &fs.entries)
	}
	if fs.entries == nil {
		fs.entries = []FlagEntry{}
	}
	return fs, nil
}

func (fs *FlagStore) save() {
	b, err := json.MarshalIndent(fs.entries, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "lazydict: flagstore marshal error: %v\n", err)
		return
	}
	if err := os.WriteFile(fs.path, b, 0600); err != nil {
		fmt.Fprintf(os.Stderr, "lazydict: flagstore write error: %v\n", err)
	}
}

// Add upserts a flag: refreshes snapshot, preserves created_at and existing note.
func (fs *FlagStore) Add(word, snapshot string) {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	for i, e := range fs.entries {
		if e.Word == word {
			fs.entries[i].Snapshot = snapshot
			// CreatedAt intentionally not updated — reflects first-flag time
			fs.save()
			return
		}
	}
	fs.entries = append(fs.entries, FlagEntry{
		Word:      word,
		Snapshot:  snapshot,
		CreatedAt: time.Now(),
	})
	fs.save()
}

// UpdateNote sets the note for a flagged word. No-op if word not flagged.
func (fs *FlagStore) UpdateNote(word, note string) {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	for i, e := range fs.entries {
		if e.Word == word {
			fs.entries[i].Note = note
			fs.save()
			return
		}
	}
}

// Delete removes the flag for a word.
func (fs *FlagStore) Delete(word string) {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	fs.entries = slices.DeleteFunc(fs.entries, func(e FlagEntry) bool { return e.Word == word })
	fs.save()
}

// All returns all flags in insertion order (created_at ascending for append-only entries).
func (fs *FlagStore) All() []FlagEntry {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	out := make([]FlagEntry, len(fs.entries))
	copy(out, fs.entries)
	return out
}

// Get returns the flag entry for a word and whether it was found.
func (fs *FlagStore) Get(word string) (FlagEntry, bool) {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	for _, e := range fs.entries {
		if e.Word == word {
			return e, true
		}
	}
	return FlagEntry{}, false
}

// Words returns flagged words in insertion order (created_at ascending for append-only entries).
func (fs *FlagStore) Words() []string {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	out := make([]string, len(fs.entries))
	for i, e := range fs.entries {
		out[i] = e.Word
	}
	return out
}

package store_test

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/peterqlin/lazydict/internal/store"
)

func newFlagStore(t *testing.T) *store.FlagStore {
	t.Helper()
	fs, err := store.NewFlagStore(filepath.Join(t.TempDir(), "flags.json"))
	if err != nil {
		t.Fatalf("NewFlagStore: %v", err)
	}
	return fs
}

func TestFlagAdd(t *testing.T) {
	fs := newFlagStore(t)
	fs.Add("ephemeral", "snapshot text")
	entries := fs.All()
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].Word != "ephemeral" || entries[0].Snapshot != "snapshot text" {
		t.Errorf("unexpected entry: %+v", entries[0])
	}
}

func TestFlagUpsertPreservesNote(t *testing.T) {
	fs := newFlagStore(t)
	fs.Add("ephemeral", "old snapshot")
	fs.UpdateNote("ephemeral", "my note")
	fs.Add("ephemeral", "new snapshot") // re-flag: refreshes snapshot, keeps note
	e, ok := fs.Get("ephemeral")
	if !ok {
		t.Fatal("entry not found")
	}
	if e.Snapshot != "new snapshot" {
		t.Errorf("expected new snapshot, got %q", e.Snapshot)
	}
	if e.Note != "my note" {
		t.Errorf("expected note preserved, got %q", e.Note)
	}
}

func TestFlagUpdateNote(t *testing.T) {
	fs := newFlagStore(t)
	fs.Add("ephemeral", "snap")
	fs.UpdateNote("ephemeral", "definition 2 is broken")
	e, _ := fs.Get("ephemeral")
	if e.Note != "definition 2 is broken" {
		t.Errorf("unexpected note: %q", e.Note)
	}
}

func TestFlagDelete(t *testing.T) {
	fs := newFlagStore(t)
	fs.Add("alpha", "snap1")
	fs.Add("beta", "snap2")
	fs.Delete("alpha")
	entries := fs.All()
	if len(entries) != 1 || entries[0].Word != "beta" {
		t.Errorf("unexpected entries after delete: %+v", entries)
	}
}

func TestFlagAllOrder(t *testing.T) {
	fs := newFlagStore(t)
	fs.Add("beta", "s")
	time.Sleep(time.Millisecond) // ensure distinct created_at
	fs.Add("alpha", "s")
	entries := fs.All()
	if entries[0].Word != "beta" || entries[1].Word != "alpha" {
		t.Errorf("expected created_at asc order, got %v %v", entries[0].Word, entries[1].Word)
	}
}

func TestFlagPersistence(t *testing.T) {
	path := filepath.Join(t.TempDir(), "flags.json")
	fs1, _ := store.NewFlagStore(path)
	fs1.Add("persist", "snap")
	fs1.UpdateNote("persist", "a note")

	fs2, err := store.NewFlagStore(path)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	e, ok := fs2.Get("persist")
	if !ok || e.Note != "a note" {
		t.Errorf("data not persisted: %+v, ok=%v", e, ok)
	}
}

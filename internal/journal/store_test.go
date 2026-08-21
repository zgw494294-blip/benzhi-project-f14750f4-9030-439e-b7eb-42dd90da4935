package journal

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"stageready/internal/domain"
)

func TestStorePersistsHashChainAndReplays(t *testing.T) {
	directory := t.TempDir()
	store, err := Open(directory)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	aggregate, event, err := domain.CreateSession(domain.CreateSessionInput{ID: "s1", ProductionName: "演出", Venue: "剧场", PerformanceDate: now, TechnicalDirector: "负责人"}, now)
	if err != nil {
		t.Fatal(err)
	}
	_ = aggregate
	commit, err := store.Append("s1", 0, "k1", []domain.Event{event})
	if err != nil {
		t.Fatal(err)
	}
	if commit.Sequence != 1 || store.HeadHash() == "" {
		t.Fatalf("unexpected commit: %#v", commit)
	}
	if duplicate, err := store.Append("s1", 0, "k1", []domain.Event{event}); err != nil || !duplicate.Duplicate {
		t.Fatalf("expected idempotent duplicate: %#v %v", duplicate, err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(directory)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	if reopened.Sequence() != 1 || reopened.SessionVersion("s1") != 1 {
		t.Fatalf("replay did not restore state")
	}
	if _, err := os.Stat(filepath.Join(directory, "events.jsonl")); err != nil {
		t.Fatal(err)
	}
}

func TestStoreRejectsCorruptedRecord(t *testing.T) {
	directory := t.TempDir()
	store, err := Open(directory)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	_, event, _ := domain.CreateSession(domain.CreateSessionInput{ID: "s1", ProductionName: "演出", Venue: "剧场", PerformanceDate: now, TechnicalDirector: "负责人"}, now)
	if _, err := store.Append("s1", 0, "k1", []domain.Event{event}); err != nil {
		t.Fatal(err)
	}
	store.Close()
	path := filepath.Join(directory, "events.jsonl")
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	contents[len(contents)-2] = '0'
	if err := os.WriteFile(path, contents, 0o640); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(directory); err == nil {
		t.Fatal("expected corruption error")
	}
}

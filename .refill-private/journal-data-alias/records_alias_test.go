package journaldataalias

import (
	"testing"
	"time"

	"stageready/internal/domain"
	"stageready/internal/journal"
)

func TestJournalReadCopiesKeepChainImmutable(t *testing.T) {
	for _, read := range []func(*journal.Store) {
		func(store *journal.Store) {
			records := store.Records()
			records[0].Event.Data[0] ^= 0xff
		},
		func(store *journal.Store) {
			events := store.Events()
			events[0].Data[0] ^= 0xff
		},
	} {
		store, err := journal.Open(t.TempDir())
		if err != nil {
			t.Fatal(err)
		}
		event, err := domain.MakeEvent(domain.EventSessionCreated, "s1", 1, time.Unix(0, 0), domain.SessionCreated{Session: domain.ValidationSession{ID: "s1"}})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := store.Append("s1", 0, "k1", []domain.Event{event}); err != nil {
			t.Fatal(err)
		}
		read(store)
		if verification := store.VerifyPrefix(1); !verification.Valid {
			t.Fatalf("journal read mutated internal chain: %#v", verification)
		}
		store.Close()
	}
}

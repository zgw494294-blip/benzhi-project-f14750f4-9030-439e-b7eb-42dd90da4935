package snapshot_projection_gap_test

import (
	"testing"
	"time"

	"stageready/internal/application"
	"stageready/internal/domain"
	"stageready/internal/journal"
)

func TestRestartRebuildsWhenProjectionSnapshotOmitsCommittedSession(t *testing.T) {
	dir := t.TempDir()
	store, err := journal.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	service, err := application.NewService(store, func() time.Time {
		return time.Date(2026, 8, 21, 10, 0, 0, 0, time.UTC)
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.CreateSession(application.CreateSessionCommand{
		CommandMeta:       application.CommandMeta{IdempotencyKey: "snapshot-gap-create"},
		ID:                "snapshot-gap-session",
		ProductionName:    "快照缺口复现",
		Venue:             "主舞台",
		PerformanceDate:   time.Date(2026, 8, 22, 0, 0, 0, 0, time.UTC),
		TechnicalDirector: "负责人",
	}); err != nil {
		t.Fatal(err)
	}

	// This is a checksum-valid but incomplete disposable projection. The event log
	// remains the source of truth and should rebuild the missing session on restart.
	emptyProjection := struct {
		Sessions map[string]*domain.Aggregate `json:"sessions"`
	}{Sessions: map[string]*domain.Aggregate{}}
	if err := store.SaveSnapshot(emptyProjection); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}

	reopened, err := journal.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	restored, err := application.NewService(reopened, func() time.Time {
		return time.Date(2026, 8, 21, 10, 0, 0, 0, time.UTC)
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = restored.Close() })
	if _, err := restored.GetSession("snapshot-gap-session"); err != nil {
		t.Fatalf("restart lost journal session from valid snapshot: %v", err)
	}
}

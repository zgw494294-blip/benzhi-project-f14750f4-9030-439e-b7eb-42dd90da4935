package snapshot_nil_projection

import (
	"testing"
	"time"

	"stageready/internal/application"
	"stageready/internal/domain"
	"stageready/internal/journal"
)

func TestNilProjectionSnapshotDoesNotCrashRestart(t *testing.T) {
	dir := t.TempDir()
	store, err := journal.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.SaveSnapshot(map[string]any{
		"sessions": map[string]*domain.Aggregate{"orphan": nil},
	}); err != nil {
		_ = store.Close()
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}

	reopened, err := journal.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()

	defer func() {
		if recovered := recover(); recovered != nil {
			t.Fatalf("nil projection snapshot panicked during restart: %v", recovered)
		}
	}()
	service, err := application.NewService(reopened, func() time.Time { return time.Date(2026, 8, 21, 10, 0, 0, 0, time.UTC) })
	if err != nil {
		t.Fatalf("restart rejected recoverable projection snapshot: %v", err)
	}
	defer service.Close()
	if _, err := service.GetSession("orphan"); err == nil {
		t.Fatal("nil projection snapshot created a phantom session")
	}
}

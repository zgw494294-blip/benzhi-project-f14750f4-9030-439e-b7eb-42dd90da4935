package createcancelcommit_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"stageready/internal/application"
	"stageready/internal/journal"
)

func TestCanceledCreateDoesNotCommitSession(t *testing.T) {
	store, err := journal.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	service, err := application.NewService(store, func() time.Time {
		return time.Date(2026, 8, 21, 10, 0, 0, 0, time.UTC)
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = service.Close() })

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = service.CreateSessionContext(ctx, application.CreateSessionCommand{
		CommandMeta:       application.CommandMeta{IdempotencyKey: "canceled-create"},
		ID:                "canceled-session",
		ProductionName:    "取消的演出",
		Venue:             "主舞台",
		PerformanceDate:   time.Date(2026, 8, 22, 19, 30, 0, 0, time.UTC),
		TechnicalDirector: "技术负责人",
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected canceled request error, got %v", err)
	}

	detail, lookupErr := service.GetSession("canceled-session")
	if lookupErr == nil {
		t.Fatalf("canceled create committed session: id=%s version=%d", detail.Session.ID, detail.Session.Version)
	}
}

package application

import (
	"testing"
	"time"

	"stageready/internal/journal"
)

func TestServiceExpectedVersionAndIdempotency(t *testing.T) {
	store, err := journal.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	service, err := NewService(store, func() time.Time { return time.Date(2026, 8, 21, 10, 0, 0, 0, time.UTC) })
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	command := CreateSessionCommand{CommandMeta: CommandMeta{IdempotencyKey: "create-1"}, ProductionName: "演出", Venue: "剧场", PerformanceDate: time.Date(2026, 8, 21, 0, 0, 0, 0, time.UTC), TechnicalDirector: "负责人"}
	first, err := service.CreateSession(command)
	if err != nil {
		t.Fatal(err)
	}
	second, err := service.CreateSession(command)
	if err != nil {
		t.Fatal(err)
	}
	if !second.Commit.Duplicate || second.Detail.Session.ID != first.Detail.Session.ID {
		t.Fatalf("expected duplicate result")
	}
	_, err = service.Prepare(SessionCommand{CommandMeta: CommandMeta{ExpectedVersion: 0, IdempotencyKey: "wrong"}, SessionID: first.Detail.Session.ID})
	if err == nil {
		t.Fatal("expected prepare rule error before configuration")
	}
}

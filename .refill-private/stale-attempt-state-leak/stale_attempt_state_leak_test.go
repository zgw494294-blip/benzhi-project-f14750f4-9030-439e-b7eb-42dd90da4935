package stale_attempt_state_leak_test

import (
	"errors"
	"testing"
	"time"

	"stageready/internal/application"
	"stageready/internal/domain"
	"stageready/internal/journal"
)

func TestRejectedAttemptDoesNotLeakIntoLiveProjection(t *testing.T) {
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

	created, err := service.CreateSession(application.CreateSessionCommand{
		CommandMeta:       application.CommandMeta{IdempotencyKey: "create-stale-attempt"},
		ID:                "stale-attempt-session",
		ProductionName:    "事务隔离演出",
		Venue:             "主舞台",
		PerformanceDate:   time.Date(2026, 8, 22, 0, 0, 0, 0, time.UTC),
		TechnicalDirector: "技术负责人",
	})
	if err != nil {
		t.Fatal(err)
	}
	version := created.Detail.Session.Version
	next := func(result application.CommandResult, commandErr error) {
		t.Helper()
		if commandErr != nil {
			t.Fatal(commandErr)
		}
		version = result.Detail.Session.Version
	}
	next(service.AddDevice(application.AddDeviceCommand{
		CommandMeta:           application.CommandMeta{ExpectedVersion: version, IdempotencyKey: "device-stale-attempt"},
		SessionID:             "stale-attempt-session",
		ID:                    "hoist-1",
		Name:                  "1 号电动葫芦",
		DeviceType:            "电动葫芦",
		RatedLoadKg:           500,
		SafeZone:              "舞台上空 A 区",
		EmergencyStopRequired: true,
	}))
	next(service.AddCue(application.AddCueCommand{
		CommandMeta:        application.CommandMeta{ExpectedVersion: version, IdempotencyKey: "cue-stale-attempt"},
		SessionID:          "stale-attempt-session",
		ID:                 "cue-rise",
		Sequence:           1,
		DeviceID:           "hoist-1",
		Action:             "上升至定位线",
		ExpectedLoadKg:     300,
		MinimumClearanceCm: 80,
		MaximumStopMs:      500,
	}))
	next(service.Prepare(application.SessionCommand{
		CommandMeta: application.CommandMeta{ExpectedVersion: version, IdempotencyKey: "prepare-stale-attempt"},
		SessionID:   "stale-attempt-session",
	}))
	next(service.StartRun(application.SessionCommand{
		CommandMeta: application.CommandMeta{ExpectedVersion: version, IdempotencyKey: "run-stale-attempt"},
		SessionID:   "stale-attempt-session",
	}))

	sequenceBefore := store.Sequence()
	_, err = service.RecordAttempt(application.RecordAttemptCommand{
		CommandMeta:         application.CommandMeta{ExpectedVersion: version - 1, IdempotencyKey: "rejected-stale-attempt"},
		SessionID:           "stale-attempt-session",
		ID:                  "attempt-must-not-leak",
		CueID:               "cue-rise",
		MeasuredLoadKg:      280,
		MeasuredClearanceCm: 95,
		MeasuredStopMs:      420,
		Operator:            "操作员",
		EvidenceNote:        "现场确认",
	})
	var conflict *journal.ConflictError
	if !errors.As(err, &conflict) {
		t.Fatalf("expected version conflict, got %v", err)
	}
	if store.Sequence() != sequenceBefore {
		t.Fatalf("rejected attempt unexpectedly reached journal: before=%d after=%d", sequenceBefore, store.Sequence())
	}

	detail, err := service.GetSession("stale-attempt-session")
	if err != nil {
		t.Fatal(err)
	}
	if len(detail.Attempts) != 0 || len(detail.Cues) != 1 || detail.Cues[0].Status != domain.CuePending {
		t.Fatalf("failed attempt polluted live projection: attempts=%d cueStatus=%s version=%d", len(detail.Attempts), detail.Cues[0].Status, detail.Session.Version)
	}
}

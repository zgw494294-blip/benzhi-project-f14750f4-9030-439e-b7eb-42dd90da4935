package idempotency_replay_collapse_test

import (
	"testing"
	"time"

	"stageready/internal/application"
	"stageready/internal/journal"
)

func TestRestartKeepsOriginalIdempotencyBoundary(t *testing.T) {
	directory := t.TempDir()
	now := time.Date(2026, 8, 21, 10, 0, 0, 0, time.UTC)
	clock := func() time.Time { return now }

	store, err := journal.Open(directory)
	if err != nil {
		t.Fatal(err)
	}
	service, err := application.NewService(store, clock)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.CreateSession(application.CreateSessionCommand{
		CommandMeta:       application.CommandMeta{IdempotencyKey: "create-replay-session"},
		ID:                "replay-session",
		ProductionName:    "重启幂等验证演出",
		Venue:             "主舞台",
		PerformanceDate:   now,
		TechnicalDirector: "技术负责人",
	}); err != nil {
		t.Fatal(err)
	}
	deviceCommand := application.AddDeviceCommand{
		CommandMeta: application.CommandMeta{ExpectedVersion: 1, IdempotencyKey: "add-replay-device"},
		SessionID:   "replay-session",
		ID:          "device-1",
		Name:        "一号吊杆",
		DeviceType:  "电动吊杆",
		RatedLoadKg: 500,
		SafeZone:    "舞台上空 A 区",
	}
	if _, err := service.AddDevice(deviceCommand); err != nil {
		t.Fatal(err)
	}
	if _, err := service.AddCue(application.AddCueCommand{
		CommandMeta:        application.CommandMeta{ExpectedVersion: 2, IdempotencyKey: "add-later-cue"},
		SessionID:          "replay-session",
		ID:                 "cue-1",
		Sequence:           1,
		DeviceID:           "device-1",
		Action:             "上升至定位线",
		ExpectedLoadKg:     200,
		MinimumClearanceCm: 80,
	}); err != nil {
		t.Fatal(err)
	}
	if err := service.Close(); err != nil {
		t.Fatal(err)
	}

	reopenedStore, err := journal.Open(directory)
	if err != nil {
		t.Fatal(err)
	}
	reopened, err := application.NewService(reopenedStore, clock)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = reopened.Close() })

	replayed, err := reopened.AddDevice(deviceCommand)
	if err != nil {
		t.Fatalf("restart collapsed idempotency boundary: %v", err)
	}
	if !replayed.Commit.Duplicate || replayed.Detail.Session.Version != 2 || len(replayed.Detail.Devices) != 1 || len(replayed.Detail.Cues) != 0 {
		t.Fatalf("restart collapsed idempotency boundary: duplicate=%t version=%d sequence=%d cueCount=%d", replayed.Commit.Duplicate, replayed.Detail.Session.Version, replayed.Commit.Sequence, len(replayed.Detail.Cues))
	}
}

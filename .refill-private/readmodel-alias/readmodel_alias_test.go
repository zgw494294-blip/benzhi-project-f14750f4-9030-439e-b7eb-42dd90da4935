package readmodelalias

import (
	"testing"
	"time"

	"stageready/internal/application"
	"stageready/internal/domain"
	"stageready/internal/journal"
)

func TestSessionDetailsAreDetachedFromServiceState(t *testing.T) {
	store, err := journal.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	service, err := application.NewService(store, func() time.Time { return time.Unix(0, 0) })
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	version := uint64(0)
	next := func(result application.CommandResult, callErr error) {
		t.Helper()
		if callErr != nil {
			t.Fatal(callErr)
		}
		version = result.Detail.Session.Version
	}
	next(service.CreateSession(application.CreateSessionCommand{CommandMeta: application.CommandMeta{ExpectedVersion: 0, IdempotencyKey: "create"}, ID: "s1", ProductionName: "演出", Venue: "场地", PerformanceDate: time.Unix(0, 0), TechnicalDirector: "负责人"}))
	next(service.AddDevice(application.AddDeviceCommand{CommandMeta: application.CommandMeta{ExpectedVersion: version, IdempotencyKey: "device"}, SessionID: "s1", ID: "d1", Name: "设备", DeviceType: "电动", RatedLoadKg: 500, SafeZone: "A"}))
	next(service.AddCue(application.AddCueCommand{CommandMeta: application.CommandMeta{ExpectedVersion: version, IdempotencyKey: "cue"}, SessionID: "s1", ID: "c1", Sequence: 1, DeviceID: "d1", Action: "上升", ExpectedLoadKg: 100, MinimumClearanceCm: 50}))
	next(service.Prepare(application.SessionCommand{CommandMeta: application.CommandMeta{ExpectedVersion: version, IdempotencyKey: "prepare"}, SessionID: "s1"}))
	next(service.StartRun(application.SessionCommand{CommandMeta: application.CommandMeta{ExpectedVersion: version, IdempotencyKey: "run"}, SessionID: "s1"}))
	next(service.RecordAttempt(application.RecordAttemptCommand{CommandMeta: application.CommandMeta{ExpectedVersion: version, IdempotencyKey: "attempt"}, SessionID: "s1", ID: "a1", CueID: "c1", MeasuredLoadKg: 600, MeasuredClearanceCm: 10, Operator: "操作员", EvidenceNote: "证据"}))
	next(service.CompleteReview(application.CompleteReviewCommand{CommandMeta: application.CommandMeta{ExpectedVersion: version, IdempotencyKey: "review"}, SessionID: "s1", ID: "r1", Reviewer: "检查员", Decision: domain.ReviewNeedsCorrection, Findings: []string{"载荷"}, CorrectionNote: "整改"}))
	detail, err := service.GetSession("s1")
	if err != nil {
		t.Fatal(err)
	}
	detail.Session.DeviceIDs[0] = "mutated-device"
	detail.Session.CueIDs[0] = "mutated-cue"
	detail.Attempts[0].Violations[0].Message = "mutated-violation"
	detail.CorrectionTasks[0].Violations[0].Message = "mutated-task"
	again, err := service.GetSession("s1")
	if err != nil {
		t.Fatal(err)
	}
	if again.Session.DeviceIDs[0] != "d1" || again.Session.CueIDs[0] != "c1" || again.Attempts[0].Violations[0].Message == "mutated-violation" || again.CorrectionTasks[0].Violations[0].Message == "mutated-task" {
		t.Fatalf("session detail aliases internal state: %#v", again)
	}
}

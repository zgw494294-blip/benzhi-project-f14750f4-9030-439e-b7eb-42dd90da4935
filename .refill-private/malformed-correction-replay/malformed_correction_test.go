package malformedcorrectionreplay

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"stageready/internal/application"
	"stageready/internal/domain"
	"stageready/internal/journal"
)

func TestMalformedCorrectionEventRejectedDuringReplay(t *testing.T) {
	dir := t.TempDir()
	store, err := journal.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	service, err := application.NewService(store, fixedClock)
	if err != nil {
		t.Fatal(err)
	}
	version := createCorrectionSession(t, service)
	malformed, err := domain.MakeEvent(domain.EventCorrectionSubmitted, "correction-session", version+1, fixedClock(), domain.CorrectionSubmitted{TaskCueIDs: []string{"ghost-cue"}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Append("correction-session", version, "malformed-correction", []domain.Event{malformed}); err != nil {
		t.Fatal(err)
	}
	if err := service.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(dir, "snapshot.json")); err != nil {
		t.Fatal(err)
	}
	reopened, err := journal.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	restored, err := application.NewService(reopened, fixedClock)
	if err == nil {
		detail, detailErr := restored.GetSession("correction-session")
		t.Fatalf("malformed correction event was accepted: detail=%#v err=%v", detail, detailErr)
	}
}

func createCorrectionSession(t *testing.T, service *application.Service) uint64 {
	t.Helper()
	date := time.Date(2026, 8, 21, 0, 0, 0, 0, time.UTC)
	result, err := service.CreateSession(application.CreateSessionCommand{CommandMeta: application.CommandMeta{IdempotencyKey: "create-correction"}, ID: "correction-session", ProductionName: "整改复演", Venue: "主舞台", PerformanceDate: date, TechnicalDirector: "负责人"})
	if err != nil {
		t.Fatal(err)
	}
	version := result.Detail.Session.Version
	result, err = service.AddDevice(application.AddDeviceCommand{CommandMeta: application.CommandMeta{ExpectedVersion: version, IdempotencyKey: "device-correction"}, SessionID: "correction-session", ID: "d1", Name: "吊杆", DeviceType: "电动", RatedLoadKg: 100, SafeZone: "A"})
	if err != nil {
		t.Fatal(err)
	}
	version = result.Detail.Session.Version
	result, err = service.AddCue(application.AddCueCommand{CommandMeta: application.CommandMeta{ExpectedVersion: version, IdempotencyKey: "cue-correction"}, SessionID: "correction-session", ID: "c1", Sequence: 1, DeviceID: "d1", Action: "上升", ExpectedLoadKg: 80, MinimumClearanceCm: 30})
	if err != nil {
		t.Fatal(err)
	}
	version = result.Detail.Session.Version
	result, err = service.Prepare(application.SessionCommand{CommandMeta: application.CommandMeta{ExpectedVersion: version, IdempotencyKey: "prepare-correction"}, SessionID: "correction-session"})
	if err != nil {
		t.Fatal(err)
	}
	version = result.Detail.Session.Version
	result, err = service.StartRun(application.SessionCommand{CommandMeta: application.CommandMeta{ExpectedVersion: version, IdempotencyKey: "run-correction"}, SessionID: "correction-session"})
	if err != nil {
		t.Fatal(err)
	}
	version = result.Detail.Session.Version
	result, err = service.RecordAttempt(application.RecordAttemptCommand{CommandMeta: application.CommandMeta{ExpectedVersion: version, IdempotencyKey: "attempt-correction"}, SessionID: "correction-session", ID: "a1", CueID: "c1", MeasuredLoadKg: 120, MeasuredClearanceCm: 40, Operator: "操作员", EvidenceNote: "现场证据"})
	if err != nil {
		t.Fatal(err)
	}
	version = result.Detail.Session.Version
	result, err = service.CompleteReview(application.CompleteReviewCommand{CommandMeta: application.CommandMeta{ExpectedVersion: version, IdempotencyKey: "review-correction"}, SessionID: "correction-session", ID: "r1", Reviewer: "检查员", Decision: domain.ReviewNeedsCorrection, Findings: []string{"载荷超限"}, CorrectionNote: "整改后复测"})
	if err != nil {
		t.Fatal(err)
	}
	return result.Detail.Session.Version
}

func fixedClock() time.Time {
	return time.Date(2026, 8, 21, 10, 0, 0, 0, time.UTC)
}

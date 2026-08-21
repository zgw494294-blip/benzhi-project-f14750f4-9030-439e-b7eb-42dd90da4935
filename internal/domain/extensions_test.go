package domain

import (
	"errors"
	"testing"
	"time"
)

func draftAggregate(t *testing.T) (*Aggregate, time.Time) {
	t.Helper()
	now := time.Date(2026, 8, 21, 10, 0, 0, 0, time.UTC)
	aggregate, _, err := CreateSession(CreateSessionInput{ID: "draft", ProductionName: "演出", Venue: "主舞台", PerformanceDate: now, TechnicalDirector: "技术负责人"}, now)
	if err != nil {
		t.Fatal(err)
	}
	return aggregate, now
}

func TestDraftRevisionChecksDependenciesAndKeepsCueOrderContinuous(t *testing.T) {
	aggregate, now := draftAggregate(t)
	if _, err := aggregate.AddDevice(RiggingDevice{ID: "d1", Name: "吊杆", DeviceType: "电动", RatedLoadKg: 500, SafeZone: "A", EmergencyStopRequired: true}, now); err != nil {
		t.Fatal(err)
	}
	for _, cue := range []SafetyCue{{ID: "c1", Sequence: 1, DeviceID: "d1", Action: "上升", ExpectedLoadKg: 300, MinimumClearanceCm: 80, MaximumStopMs: 500}, {ID: "c2", Sequence: 2, DeviceID: "d1", Action: "下降", ExpectedLoadKg: 200, MinimumClearanceCm: 70, MaximumStopMs: 450}} {
		if _, err := aggregate.AddCue(cue, now); err != nil {
			t.Fatal(err)
		}
	}
	version := aggregate.Session.Version
	_, err := aggregate.UpdateDevice(RiggingDevice{ID: "d1", Name: "吊杆", DeviceType: "电动", RatedLoadKg: 250, SafeZone: "A", EmergencyStopRequired: true}, now)
	var validation *ValidationError
	if !errors.As(err, &validation) || validation.Problems[0].ID != "c1" || aggregate.Session.Version != version || aggregate.Devices["d1"].RatedLoadKg != 500 {
		t.Fatalf("expected atomic dependency conflict, got %#v / %v", validation, err)
	}
	events, err := aggregate.UpdateCue(SafetyCue{ID: "c2", Sequence: 1, DeviceID: "d1", Action: "优先下降", ExpectedLoadKg: 200, MinimumClearanceCm: 70, MaximumStopMs: 450}, now)
	if err != nil || len(events) != 2 || aggregate.OrderedCues()[0].ID != "c2" {
		t.Fatalf("expected update and atomic reorder: %v %#v", err, aggregate.OrderedCues())
	}
	if _, err := aggregate.DeleteCue("c2", now); err != nil || aggregate.Cues["c1"].Sequence != 1 {
		t.Fatalf("delete did not compact sequence: %v", err)
	}
	if _, err := aggregate.DeleteDevice("d1", now); err == nil {
		t.Fatal("expected referenced device deletion to fail")
	}
	if _, err := aggregate.Prepare(now); err != nil {
		t.Fatal(err)
	}
	if _, err := aggregate.DeleteCue("c1", now); err == nil {
		t.Fatal("expected Prepared baseline to be frozen")
	}
}

func TestConfigurationBatchPreflightAndConfirmationAreAtomic(t *testing.T) {
	aggregate, now := draftAggregate(t)
	invalid := BatchConfigurationInput{Devices: []RiggingDevice{{ID: "d1", Name: "吊杆", DeviceType: "电动", RatedLoadKg: 200, SafeZone: "A"}}, Cues: []SafetyCue{{ID: "c1", Sequence: 2, DeviceID: "d1", Action: "上升", ExpectedLoadKg: 300, MinimumClearanceCm: 50}}}
	version := aggregate.Session.Version
	if _, _, err := aggregate.ConfirmConfigurationBatch(invalid, now); err == nil || aggregate.Session.Version != version || len(aggregate.Devices) != 0 {
		t.Fatalf("invalid batch changed aggregate: %v", err)
	}
	valid := BatchConfigurationInput{Devices: []RiggingDevice{{ID: "d2", Name: "二号", DeviceType: "电动", RatedLoadKg: 400, SafeZone: "B"}, {ID: "d1", Name: "一号", DeviceType: "电动", RatedLoadKg: 500, SafeZone: "A", EmergencyStopRequired: true}}, Cues: []SafetyCue{{ID: "c2", Sequence: 2, DeviceID: "d2", Action: "下降", ExpectedLoadKg: 200, MinimumClearanceCm: 60}, {ID: "c1", Sequence: 1, DeviceID: "d1", Action: "上升", ExpectedLoadKg: 300, MinimumClearanceCm: 80, MaximumStopMs: 500}}}
	events, report, err := aggregate.ConfirmConfigurationBatch(valid, now)
	if err != nil || !report.Valid || len(events) != 4 || events[0].Type != EventDeviceAdded || aggregate.OrderedCues()[1].ID != "c2" {
		t.Fatalf("unexpected valid batch: %#v %v", report, err)
	}
}

func TestAttemptBatchRejectsOutOfOrderWithoutPartialMutation(t *testing.T) {
	aggregate, now := draftAggregate(t)
	_, _ = aggregate.AddDevice(RiggingDevice{ID: "d1", Name: "吊杆", DeviceType: "电动", RatedLoadKg: 500, SafeZone: "A"}, now)
	_, _ = aggregate.AddCue(SafetyCue{ID: "c1", Sequence: 1, DeviceID: "d1", Action: "一", ExpectedLoadKg: 100, MinimumClearanceCm: 50}, now)
	_, _ = aggregate.AddCue(SafetyCue{ID: "c2", Sequence: 2, DeviceID: "d1", Action: "二", ExpectedLoadKg: 100, MinimumClearanceCm: 50}, now)
	_, _ = aggregate.Prepare(now)
	_, _ = aggregate.StartRun(now)
	version := aggregate.Session.Version
	bad := []RecordAttemptInput{{ID: "a2", CueID: "c2", MeasuredLoadKg: 100, MeasuredClearanceCm: 60, Operator: "操作员", EvidenceNote: "证据"}}
	if _, err := aggregate.RecordAttemptBatch(bad, now); err == nil || aggregate.Session.Version != version || len(aggregate.Attempts) != 0 {
		t.Fatalf("out-of-order batch was not atomic: %v", err)
	}
	good := []RecordAttemptInput{{ID: "a1", CueID: "c1", MeasuredLoadKg: 100, MeasuredClearanceCm: 60, Operator: "甲", EvidenceNote: "证据一"}, {ID: "a2", CueID: "c2", MeasuredLoadKg: 600, MeasuredClearanceCm: 40, Operator: "乙", EvidenceNote: "证据二"}}
	events, err := aggregate.RecordAttemptBatch(good, now)
	if err != nil || len(events) != 3 || events[2].Type != EventReviewRequested || aggregate.Session.Status != SessionReview || aggregate.Cues["c2"].Status != CueFailed {
		t.Fatalf("unexpected batch result: %v %#v", err, events)
	}
}

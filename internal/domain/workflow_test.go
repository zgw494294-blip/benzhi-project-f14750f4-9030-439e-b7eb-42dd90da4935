package domain

import (
	"testing"
	"time"
)

func testAggregate(t *testing.T) *Aggregate {
	t.Helper()
	now := time.Date(2026, 8, 21, 10, 0, 0, 0, time.UTC)
	aggregate, _, err := CreateSession(CreateSessionInput{ID: "s1", ProductionName: "演出", Venue: "剧场", PerformanceDate: now, TechnicalDirector: "负责人"}, now)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := aggregate.AddDevice(RiggingDevice{ID: "d1", Name: "吊杆", DeviceType: "电动葫芦", RatedLoadKg: 500, SafeZone: "A区", EmergencyStopRequired: true}, now); err != nil {
		t.Fatal(err)
	}
	if _, err := aggregate.AddCue(SafetyCue{ID: "c1", Sequence: 1, DeviceID: "d1", Action: "上升", ExpectedLoadKg: 300, MinimumClearanceCm: 80, MaximumStopMs: 500}, now); err != nil {
		t.Fatal(err)
	}
	if _, err := aggregate.Prepare(now); err != nil {
		t.Fatal(err)
	}
	if _, err := aggregate.StartRun(now); err != nil {
		t.Fatal(err)
	}
	return aggregate
}

func TestAttemptViolationsAreStructured(t *testing.T) {
	aggregate := testAggregate(t)
	events, err := aggregate.RecordAttempt(RecordAttemptInput{ID: "a1", CueID: "c1", MeasuredLoadKg: 600, MeasuredClearanceCm: 40, MeasuredStopMs: 900, Operator: "操作员", EvidenceNote: "现场记录"}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 2 || aggregate.Session.Status != SessionReview {
		t.Fatalf("expected attempt and review events, got %d / %s", len(events), aggregate.Session.Status)
	}
	attempt := aggregate.Attempts["c1"][0]
	if attempt.Result != AttemptFail || len(attempt.Violations) != 3 {
		t.Fatalf("unexpected result: %#v", attempt)
	}
	if attempt.Violations[0].Code != ViolationLoad || attempt.Violations[1].Code != ViolationClearance || attempt.Violations[2].Code != ViolationStopTime {
		t.Fatalf("unexpected violation codes: %#v", attempt.Violations)
	}
}

func TestCorrectionReopensOnlyFailedCue(t *testing.T) {
	aggregate := testAggregate(t)
	if _, err := aggregate.RecordAttempt(RecordAttemptInput{ID: "a1", CueID: "c1", MeasuredLoadKg: 600, MeasuredClearanceCm: 40, MeasuredStopMs: 900, Operator: "操作员", EvidenceNote: "失败记录"}, time.Now()); err != nil {
		t.Fatal(err)
	}
	if _, err := aggregate.CompleteReview(CompleteReviewInput{ID: "r1", Reviewer: "检查员", Decision: ReviewNeedsCorrection, Findings: []string{"净空不足"}, CorrectionNote: "调整警戒线"}, time.Now()); err != nil {
		t.Fatal(err)
	}
	if aggregate.Session.Status != SessionCorrection || !aggregate.CorrectionCueIDs["c1"] {
		t.Fatalf("expected correction state")
	}
	if _, err := aggregate.UpdateCorrectionTask(UpdateCorrectionTaskInput{CueID: "c1", Measure: "调整机械限位", Owner: "负责人", EvidenceNote: "复测照片", Closed: true}, time.Now()); err != nil {
		t.Fatal(err)
	}
	if _, err := aggregate.SubmitCorrection("已调整", time.Now()); err != nil {
		t.Fatal(err)
	}
	if aggregate.Session.Status != SessionRunning || aggregate.Cues["c1"].Status != CuePending {
		t.Fatalf("expected failed cue reopened")
	}
}

func TestCertificateDigestIsRepeatable(t *testing.T) {
	now := time.Date(2026, 8, 21, 10, 0, 0, 0, time.UTC)
	certificate := ReadinessCertificate{ID: "cert", SessionID: "s", IssuedAt: now, Reviewer: "r", SessionVersion: 10, EventHeadHash: "abc"}
	certificate.Digest = CertificateDigest(certificate)
	if !VerifyCertificate(certificate) {
		t.Fatal("expected valid certificate")
	}
	certificate.Reviewer = "other"
	if VerifyCertificate(certificate) {
		t.Fatal("expected changed certificate to fail verification")
	}
}

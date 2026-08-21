package application

import (
	"testing"
	"time"

	"stageready/internal/domain"
	"stageready/internal/journal"
)

func extensionService(t *testing.T) *Service {
	t.Helper()
	store, err := journal.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	service, err := NewService(store, func() time.Time { return time.Date(2026, 8, 21, 10, 0, 0, 0, time.UTC) })
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = service.Close() })
	return service
}

func certifiedSession(t *testing.T, service *Service) CommandResult {
	t.Helper()
	now := time.Date(2026, 8, 22, 0, 0, 0, 0, time.UTC)
	created, err := service.CreateSession(CreateSessionCommand{CommandMeta: CommandMeta{IdempotencyKey: "create-cert"}, ID: "cert-session", ProductionName: "认证演出", Venue: "主舞台", PerformanceDate: now, TechnicalDirector: "总监"})
	if err != nil {
		t.Fatal(err)
	}
	version := created.Detail.Session.Version
	next := func(result CommandResult, err error) CommandResult {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
		version = result.Detail.Session.Version
		return result
	}
	next(service.AddDevice(AddDeviceCommand{CommandMeta: CommandMeta{ExpectedVersion: version, IdempotencyKey: "device-cert"}, SessionID: "cert-session", ID: "d1", Name: "吊杆", DeviceType: "电动", RatedLoadKg: 500, SafeZone: "A"}))
	next(service.AddCue(AddCueCommand{CommandMeta: CommandMeta{ExpectedVersion: version, IdempotencyKey: "cue-cert"}, SessionID: "cert-session", ID: "c1", Sequence: 1, DeviceID: "d1", Action: "上升", ExpectedLoadKg: 100, MinimumClearanceCm: 50}))
	next(service.Prepare(SessionCommand{CommandMeta: CommandMeta{ExpectedVersion: version, IdempotencyKey: "prepare-cert"}, SessionID: "cert-session"}))
	next(service.StartRun(SessionCommand{CommandMeta: CommandMeta{ExpectedVersion: version, IdempotencyKey: "run-cert"}, SessionID: "cert-session"}))
	next(service.RecordAttempt(RecordAttemptCommand{CommandMeta: CommandMeta{ExpectedVersion: version, IdempotencyKey: "attempt-cert"}, SessionID: "cert-session", ID: "a1", CueID: "c1", MeasuredLoadKg: 100, MeasuredClearanceCm: 60, Operator: "操作员", EvidenceNote: "现场证据"}))
	next(service.CompleteReview(CompleteReviewCommand{CommandMeta: CommandMeta{ExpectedVersion: version, IdempotencyKey: "review-cert"}, SessionID: "cert-session", ID: "r1", Reviewer: "检查员", Decision: domain.ReviewApproved}))
	return next(service.IssueCertificate(IssueCertificateCommand{CommandMeta: CommandMeta{ExpectedVersion: version, IdempotencyKey: "issue-cert"}, SessionID: "cert-session", ID: "CERT-001"}))
}

func TestSessionQueueFiltersSummarizesAndValidates(t *testing.T) {
	service := extensionService(t)
	certifiedSession(t, service)
	if _, err := service.CreateSession(CreateSessionCommand{CommandMeta: CommandMeta{IdempotencyKey: "create-draft"}, ID: "draft-session", ProductionName: "巡演装台", Venue: "黑匣子", PerformanceDate: time.Date(2026, 8, 25, 0, 0, 0, 0, time.UTC), TechnicalDirector: "副总监"}); err != nil {
		t.Fatal(err)
	}
	page, err := service.QuerySessions(SessionQuery{Statuses: []domain.SessionStatus{domain.SessionCertified}, Venue: "主舞", ProductionQuery: "认证", Page: 1, PageSize: 10, Sort: "performanceDate", Order: "asc"})
	if err != nil || page.Total != 1 || page.Summary.CertificateCount != 1 || page.Summary.StatusCounts[domain.SessionCertified] != 1 {
		t.Fatalf("unexpected filtered queue: %#v %v", page, err)
	}
	if _, err := service.QuerySessions(SessionQuery{Statuses: []domain.SessionStatus{"Unknown"}, Page: 1, PageSize: 20}); err == nil {
		t.Fatal("expected invalid status query")
	}
	if empty, err := service.QuerySessions(SessionQuery{Venue: "不存在", Page: 1, PageSize: 20}); err != nil || empty.Sessions == nil || empty.Total != 0 {
		t.Fatalf("expected renderable empty page: %#v %v", empty, err)
	}
}

func TestCertificateVerificationIsReadOnlyAndChecksProvidedValues(t *testing.T) {
	service := extensionService(t)
	issued := certifiedSession(t, service)
	version := issued.Detail.Session.Version
	certificate := issued.Detail.Certificate
	report, err := service.VerifyCertificate("cert-session", certificate.ID, CertificateComparison{})
	if err != nil || !report.Valid || report.IssuedEventSequence == 0 || !report.Chain.Valid {
		t.Fatalf("expected valid certificate chain: %#v %v", report, err)
	}
	wrong := "wrong-digest"
	report, err = service.VerifyCertificate("cert-session", certificate.ID, CertificateComparison{Digest: &wrong})
	if err != nil || report.Valid || report.FirstFailureSequence != report.IssuedEventSequence {
		t.Fatalf("expected comparison failure at issued event: %#v %v", report, err)
	}
	detail, err := service.GetSession("cert-session")
	if err != nil || detail.Session.Version != version || detail.Session.Status != domain.SessionCertified {
		t.Fatalf("verification changed certified session: %#v %v", detail.Session, err)
	}
}

func TestIdempotentReplayReturnsOriginalProjection(t *testing.T) {
	service := extensionService(t)
	command := CreateSessionCommand{CommandMeta: CommandMeta{IdempotencyKey: "original-create"}, ID: "original", ProductionName: "原始", Venue: "A", PerformanceDate: time.Now(), TechnicalDirector: "负责人"}
	created, err := service.CreateSession(command)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.AddDevice(AddDeviceCommand{CommandMeta: CommandMeta{ExpectedVersion: 1, IdempotencyKey: "later-device"}, SessionID: "original", ID: "d1", Name: "设备", DeviceType: "电动", RatedLoadKg: 100, SafeZone: "A"}); err != nil {
		t.Fatal(err)
	}
	replayed, err := service.CreateSession(command)
	if err != nil || !replayed.Commit.Duplicate || replayed.Detail.Session.Version != created.Detail.Session.Version || len(replayed.Detail.Devices) != 0 {
		t.Fatalf("expected original command projection: %#v %v", replayed, err)
	}
}

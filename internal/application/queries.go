package application

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"stageready/internal/domain"
	"stageready/internal/journal"
)

type SessionQuery struct {
	Statuses          []domain.SessionStatus
	Venue             string
	TechnicalDirector string
	PerformanceFrom   *time.Time
	PerformanceTo     *time.Time
	ProductionQuery   string
	Sort              string
	Order             string
	Page              int
	PageSize          int
}

type RiskSummary struct {
	StatusCounts     map[domain.SessionStatus]int `json:"statusCounts"`
	PendingCueCount  int                          `json:"pendingCueCount"`
	FailedCueCount   int                          `json:"failedCueCount"`
	CertificateCount int                          `json:"certificateCount"`
}

type SessionPage struct {
	Sessions []SessionSummary `json:"sessions"`
	Summary  RiskSummary      `json:"summary"`
	Page     int              `json:"page"`
	PageSize int              `json:"pageSize"`
	Total    int              `json:"total"`
	Pages    int              `json:"pages"`
}

func (s *Service) QuerySessions(query SessionQuery) (SessionPage, error) {
	if query.Page == 0 {
		query.Page = 1
	}
	if query.PageSize == 0 {
		query.PageSize = 20
	}
	if query.Sort == "" {
		query.Sort = "createdAt"
	}
	if query.Order == "" {
		query.Order = "desc"
	}
	allowedSort := map[string]bool{"createdAt": true, "performanceDate": true, "productionName": true, "venue": true, "status": true, "failedCount": true}
	if !allowedSort[query.Sort] {
		return SessionPage{}, &RequestError{Code: "INVALID_SORT", Message: "sort 必须是 createdAt、performanceDate、productionName、venue、status 或 failedCount", Field: "sort"}
	}
	if query.Order != "asc" && query.Order != "desc" {
		return SessionPage{}, &RequestError{Code: "INVALID_ORDER", Message: "order 必须是 asc 或 desc", Field: "order"}
	}
	if query.Page < 1 {
		return SessionPage{}, &RequestError{Code: "INVALID_PAGE", Message: "page 必须大于等于 1", Field: "page"}
	}
	if query.PageSize < 1 || query.PageSize > 100 {
		return SessionPage{}, &RequestError{Code: "INVALID_PAGE_SIZE", Message: "pageSize 必须在 1 到 100 之间", Field: "pageSize"}
	}
	if query.PerformanceFrom != nil && query.PerformanceTo != nil && query.PerformanceFrom.After(*query.PerformanceTo) {
		return SessionPage{}, &RequestError{Code: "INVALID_DATE_RANGE", Message: "performanceFrom 不能晚于 performanceTo", Field: "performanceFrom"}
	}
	validStatuses := map[domain.SessionStatus]bool{domain.SessionDraft: true, domain.SessionPrepared: true, domain.SessionRunning: true, domain.SessionReview: true, domain.SessionCorrection: true, domain.SessionCertified: true}
	statusSet := make(map[domain.SessionStatus]bool)
	for _, status := range query.Statuses {
		if !validStatuses[status] {
			return SessionPage{}, &RequestError{Code: "INVALID_STATUS", Message: fmt.Sprintf("未知会话状态 %s", status), Field: "status"}
		}
		statusSet[status] = true
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	filtered := make([]SessionSummary, 0, len(s.sessions))
	summary := RiskSummary{StatusCounts: make(map[domain.SessionStatus]int)}
	venue := strings.ToLower(strings.TrimSpace(query.Venue))
	director := strings.ToLower(strings.TrimSpace(query.TechnicalDirector))
	production := strings.ToLower(strings.TrimSpace(query.ProductionQuery))
	for _, aggregate := range s.sessions {
		session := aggregate.Session
		if len(statusSet) > 0 && !statusSet[session.Status] || venue != "" && !strings.Contains(strings.ToLower(session.Venue), venue) || director != "" && !strings.Contains(strings.ToLower(session.TechnicalDirector), director) || production != "" && !strings.Contains(strings.ToLower(session.ProductionName), production) {
			continue
		}
		date := dateOnly(session.PerformanceDate)
		if query.PerformanceFrom != nil && date.Before(dateOnly(*query.PerformanceFrom)) || query.PerformanceTo != nil && date.After(dateOnly(*query.PerformanceTo)) {
			continue
		}
		item := summaryOf(aggregate)
		filtered = append(filtered, item)
		summary.StatusCounts[session.Status]++
		for _, cue := range aggregate.Cues {
			if cue.Status == domain.CuePending {
				summary.PendingCueCount++
			}
			if cue.Status == domain.CueFailed {
				summary.FailedCueCount++
			}
		}
		if aggregate.Certificate != nil {
			summary.CertificateCount++
		}
	}
	direction := 1
	if query.Order == "desc" {
		direction = -1
	}
	slices.SortFunc(filtered, func(left, right SessionSummary) int {
		comparison := compareSummary(left, right, query.Sort)
		if comparison == 0 {
			comparison = strings.Compare(left.ID, right.ID)
		}
		return comparison * direction
	})
	total := len(filtered)
	pages := 0
	if total > 0 {
		pages = (total + query.PageSize - 1) / query.PageSize
	}
	start := (query.Page - 1) * query.PageSize
	if start > total {
		start = total
	}
	end := min(start+query.PageSize, total)
	items := slices.Clone(filtered[start:end])
	if items == nil {
		items = []SessionSummary{}
	}
	return SessionPage{Sessions: items, Summary: summary, Page: query.Page, PageSize: query.PageSize, Total: total, Pages: pages}, nil
}

func dateOnly(value time.Time) time.Time {
	year, month, day := value.UTC().Date()
	return time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
}

func compareSummary(left, right SessionSummary, field string) int {
	switch field {
	case "performanceDate":
		return left.PerformanceDate.Compare(right.PerformanceDate)
	case "productionName":
		return strings.Compare(left.ProductionName, right.ProductionName)
	case "venue":
		return strings.Compare(left.Venue, right.Venue)
	case "status":
		return strings.Compare(string(left.Status), string(right.Status))
	case "failedCount":
		return left.FailedCount - right.FailedCount
	default:
		return left.CreatedAt.Compare(right.CreatedAt)
	}
}

type CertificateComparison struct {
	Digest         *string
	EventHeadHash  *string
	SessionVersion *uint64
}

type VerificationCheck struct {
	Name     string `json:"name"`
	Passed   bool   `json:"passed"`
	Expected string `json:"expected,omitempty"`
	Actual   string `json:"actual,omitempty"`
	Message  string `json:"message"`
}

type CertificateVerificationReport struct {
	Valid                bool                      `json:"valid"`
	SessionID            string                    `json:"sessionID"`
	CertificateID        string                    `json:"certificateID"`
	IssuedEventSequence  uint64                    `json:"issuedEventSequence"`
	FirstFailureSequence uint64                    `json:"firstFailureSequence,omitempty"`
	Checks               []VerificationCheck       `json:"checks"`
	Chain                journal.ChainVerification `json:"chain"`
}

func (s *Service) VerifyCertificate(sessionID, certificateID string, comparison CertificateComparison) (CertificateVerificationReport, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	aggregate := s.sessions[sessionID]
	if aggregate == nil {
		return CertificateVerificationReport{}, &NotFoundError{Resource: "session", ID: sessionID}
	}
	if aggregate.Certificate == nil || aggregate.Certificate.ID != certificateID {
		return CertificateVerificationReport{}, &NotFoundError{Resource: "certificate", ID: certificateID}
	}
	certificate := *aggregate.Certificate
	report := CertificateVerificationReport{Valid: true, SessionID: sessionID, CertificateID: certificateID, Checks: []VerificationCheck{}}
	add := func(name string, passed bool, expected, actual, message string, sequence uint64) {
		report.Checks = append(report.Checks, VerificationCheck{Name: name, Passed: passed, Expected: expected, Actual: actual, Message: message})
		if !passed {
			report.Valid = false
			if report.FirstFailureSequence == 0 && sequence > 0 {
				report.FirstFailureSequence = sequence
			}
		}
	}
	computedDigest := domain.CertificateDigest(certificate)
	add("certificate.digest", certificate.Digest == computedDigest, computedDigest, certificate.Digest, "重新计算证书摘要", 0)
	records := s.journal.Records()
	var issued journal.Record
	var payload domain.CertificateIssued
	for _, record := range records {
		if record.Event.SessionID != sessionID || record.Event.Type != domain.EventCertificateIssued {
			continue
		}
		decoded, err := domain.DecodeEvent[domain.CertificateIssued](record.Event)
		if err == nil && decoded.Certificate.ID == certificateID {
			issued = record
			payload = decoded
			break
		}
	}
	found := issued.Sequence > 0
	add("certificate.issued.event", found, certificateID, payload.Certificate.ID, "定位 certificate.issued 事件", 0)
	if found {
		report.IssuedEventSequence = issued.Sequence
		add("event.sessionID", payload.Certificate.SessionID == certificate.SessionID && issued.Event.SessionID == certificate.SessionID, certificate.SessionID, payload.Certificate.SessionID, "证书会话与事件载荷一致", issued.Sequence)
		add("event.sessionVersion", payload.Certificate.SessionVersion == certificate.SessionVersion && issued.Event.Version == certificate.SessionVersion, fmt.Sprint(certificate.SessionVersion), fmt.Sprint(issued.Event.Version), "证书版本与签发事件版本一致", issued.Sequence)
		add("event.reviewer", payload.Certificate.Reviewer == certificate.Reviewer, certificate.Reviewer, payload.Certificate.Reviewer, "检查员与签发事件载荷一致", issued.Sequence)
		add("event.issuedAt", payload.Certificate.IssuedAt.Equal(certificate.IssuedAt), certificate.IssuedAt.UTC().Format(time.RFC3339Nano), payload.Certificate.IssuedAt.UTC().Format(time.RFC3339Nano), "签发时间与事件载荷一致", issued.Sequence)
		report.Chain = s.journal.VerifyPrefix(issued.Sequence - 1)
		add("event.chain", report.Chain.Valid, "签发前链完整", report.Chain.Message, "验证 sequence、previousHash、checksum 和会话版本连续性", report.Chain.FirstFailureSequence)
		expectedHead := ""
		if issued.Sequence > 1 {
			expectedHead = records[issued.Sequence-2].Checksum
		}
		add("certificate.eventHeadHash", certificate.EventHeadHash == expectedHead && issued.PreviousHash == expectedHead, expectedHead, certificate.EventHeadHash, "EventHeadHash 正好绑定签发前链头", issued.Sequence)
	}
	if comparison.Digest != nil {
		add("provided.digest", *comparison.Digest == certificate.Digest, certificate.Digest, *comparison.Digest, "用户提供的 digest 比对", report.IssuedEventSequence)
	}
	if comparison.EventHeadHash != nil {
		add("provided.eventHeadHash", *comparison.EventHeadHash == certificate.EventHeadHash, certificate.EventHeadHash, *comparison.EventHeadHash, "用户提供的 EventHeadHash 比对", report.IssuedEventSequence)
	}
	if comparison.SessionVersion != nil {
		add("provided.sessionVersion", *comparison.SessionVersion == certificate.SessionVersion, fmt.Sprint(certificate.SessionVersion), fmt.Sprint(*comparison.SessionVersion), "用户提供的 SessionVersion 比对", report.IssuedEventSequence)
	}
	if !report.Valid && report.FirstFailureSequence == 0 && report.IssuedEventSequence > 0 {
		report.FirstFailureSequence = report.IssuedEventSequence
	}
	return report, nil
}

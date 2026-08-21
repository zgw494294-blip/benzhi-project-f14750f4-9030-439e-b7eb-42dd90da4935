package domain

import "time"

type ValidationSession struct {
	ID                string        `json:"id"`
	ProductionName    string        `json:"productionName"`
	Venue             string        `json:"venue"`
	PerformanceDate   time.Time     `json:"performanceDate"`
	TechnicalDirector string        `json:"technicalDirector"`
	Status            SessionStatus `json:"status"`
	Version           uint64        `json:"version"`
	DeviceIDs         []string      `json:"deviceIDs"`
	CueIDs            []string      `json:"cueIDs"`
	CreatedAt         time.Time     `json:"createdAt"`
	CertifiedAt       *time.Time    `json:"certifiedAt,omitempty"`
}

type RiggingDevice struct {
	ID                    string  `json:"id"`
	SessionID             string  `json:"sessionID"`
	Name                  string  `json:"name"`
	DeviceType            string  `json:"deviceType"`
	RatedLoadKg           float64 `json:"ratedLoadKg"`
	SafeZone              string  `json:"safeZone"`
	EmergencyStopRequired bool    `json:"emergencyStopRequired"`
}

type SafetyCue struct {
	ID                 string    `json:"id"`
	SessionID          string    `json:"sessionID"`
	Sequence           int       `json:"sequence"`
	DeviceID           string    `json:"deviceID"`
	Action             string    `json:"action"`
	ExpectedLoadKg     float64   `json:"expectedLoadKg"`
	MinimumClearanceCm float64   `json:"minimumClearanceCm"`
	MaximumStopMs      int       `json:"maximumStopMs"`
	Status             CueStatus `json:"status"`
	AttemptCount       int       `json:"attemptCount"`
}

type CueAttempt struct {
	ID                  string        `json:"id"`
	CueID               string        `json:"cueID"`
	AttemptNo           int           `json:"attemptNo"`
	MeasuredLoadKg      float64       `json:"measuredLoadKg"`
	MeasuredClearanceCm float64       `json:"measuredClearanceCm"`
	MeasuredStopMs      int           `json:"measuredStopMs"`
	Operator            string        `json:"operator"`
	EvidenceNote        string        `json:"evidenceNote"`
	Result              AttemptResult `json:"result"`
	Violations          []Violation   `json:"violations"`
	RecordedAt          time.Time     `json:"recordedAt"`
}

type SafetyReview struct {
	ID             string         `json:"id"`
	SessionID      string         `json:"sessionID"`
	Reviewer       string         `json:"reviewer"`
	Decision       ReviewDecision `json:"decision"`
	Findings       []string       `json:"findings"`
	CorrectionNote string         `json:"correctionNote"`
	ReviewedAt     time.Time      `json:"reviewedAt"`
}

type CorrectionTask struct {
	CueID        string      `json:"cueID"`
	AttemptID    string      `json:"attemptID"`
	Violations   []Violation `json:"violations"`
	Measure      string      `json:"measure"`
	Owner        string      `json:"owner"`
	EvidenceNote string      `json:"evidenceNote"`
	ClosedAt     *time.Time  `json:"closedAt,omitempty"`
	UpdatedAt    time.Time   `json:"updatedAt"`
}

type ReadinessCertificate struct {
	ID             string    `json:"id"`
	SessionID      string    `json:"sessionID"`
	IssuedAt       time.Time `json:"issuedAt"`
	Reviewer       string    `json:"reviewer"`
	SessionVersion uint64    `json:"sessionVersion"`
	EventHeadHash  string    `json:"eventHeadHash"`
	Digest         string    `json:"digest"`
}

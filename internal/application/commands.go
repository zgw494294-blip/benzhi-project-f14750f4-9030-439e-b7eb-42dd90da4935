package application

import (
	"time"

	"stageready/internal/domain"
)

type CommandMeta struct {
	ExpectedVersion uint64 `json:"expectedVersion"`
	IdempotencyKey  string `json:"idempotencyKey"`
}

type CreateSessionCommand struct {
	CommandMeta
	ID                string    `json:"id"`
	ProductionName    string    `json:"productionName"`
	Venue             string    `json:"venue"`
	PerformanceDate   time.Time `json:"performanceDate"`
	TechnicalDirector string    `json:"technicalDirector"`
}

type AddDeviceCommand struct {
	CommandMeta
	SessionID             string  `json:"-"`
	ID                    string  `json:"id"`
	Name                  string  `json:"name"`
	DeviceType            string  `json:"deviceType"`
	RatedLoadKg           float64 `json:"ratedLoadKg"`
	SafeZone              string  `json:"safeZone"`
	EmergencyStopRequired bool    `json:"emergencyStopRequired"`
}

type UpdateDeviceCommand = AddDeviceCommand

type DeleteDeviceCommand struct {
	CommandMeta
	SessionID string `json:"-"`
	DeviceID  string `json:"-"`
}

type AddCueCommand struct {
	CommandMeta
	SessionID          string  `json:"-"`
	ID                 string  `json:"id"`
	Sequence           int     `json:"sequence"`
	DeviceID           string  `json:"deviceID"`
	Action             string  `json:"action"`
	ExpectedLoadKg     float64 `json:"expectedLoadKg"`
	MinimumClearanceCm float64 `json:"minimumClearanceCm"`
	MaximumStopMs      int     `json:"maximumStopMs"`
}

type UpdateCueCommand = AddCueCommand

type DeleteCueCommand struct {
	CommandMeta
	SessionID string `json:"-"`
	CueID     string `json:"-"`
}

type ReorderCuesCommand struct {
	CommandMeta
	SessionID string   `json:"-"`
	CueIDs    []string `json:"cueIDs"`
}

type ConfigurationBatchCommand struct {
	CommandMeta
	SessionID string                 `json:"-"`
	Devices   []domain.RiggingDevice `json:"devices"`
	Cues      []domain.SafetyCue     `json:"cues"`
}

type ConfigurationPreflightInput struct {
	Devices []domain.RiggingDevice `json:"devices"`
	Cues    []domain.SafetyCue     `json:"cues"`
}

type SessionCommand struct {
	CommandMeta
	SessionID string `json:"-"`
}

type RecordAttemptCommand struct {
	CommandMeta
	SessionID           string  `json:"-"`
	ID                  string  `json:"id"`
	CueID               string  `json:"cueID"`
	MeasuredLoadKg      float64 `json:"measuredLoadKg"`
	MeasuredClearanceCm float64 `json:"measuredClearanceCm"`
	MeasuredStopMs      int     `json:"measuredStopMs"`
	Operator            string  `json:"operator"`
	EvidenceNote        string  `json:"evidenceNote"`
}

type RecordAttemptItem struct {
	ID                  string  `json:"id"`
	CueID               string  `json:"cueID"`
	MeasuredLoadKg      float64 `json:"measuredLoadKg"`
	MeasuredClearanceCm float64 `json:"measuredClearanceCm"`
	MeasuredStopMs      int     `json:"measuredStopMs"`
	Operator            string  `json:"operator"`
	EvidenceNote        string  `json:"evidenceNote"`
}

type RecordAttemptBatchCommand struct {
	CommandMeta
	SessionID string              `json:"-"`
	Attempts  []RecordAttemptItem `json:"attempts"`
}

type CompleteReviewCommand struct {
	CommandMeta
	SessionID      string                `json:"-"`
	ID             string                `json:"id"`
	Reviewer       string                `json:"reviewer"`
	Decision       domain.ReviewDecision `json:"decision"`
	Findings       []string              `json:"findings"`
	CorrectionNote string                `json:"correctionNote"`
}

type SubmitCorrectionCommand struct {
	CommandMeta
	SessionID string `json:"-"`
	Note      string `json:"note"`
}

type UpdateCorrectionTaskCommand struct {
	CommandMeta
	SessionID    string `json:"-"`
	CueID        string `json:"-"`
	Measure      string `json:"measure"`
	Owner        string `json:"owner"`
	EvidenceNote string `json:"evidenceNote"`
	Closed       bool   `json:"closed"`
}

type IssueCertificateCommand struct {
	CommandMeta
	SessionID string `json:"-"`
	ID        string `json:"id"`
}

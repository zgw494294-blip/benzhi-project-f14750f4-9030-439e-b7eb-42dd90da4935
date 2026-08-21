package application

import (
	"encoding/json"
	"slices"
	"time"

	"stageready/internal/domain"
	"stageready/internal/journal"
)

type SessionSummary struct {
	ID                string               `json:"id"`
	ProductionName    string               `json:"productionName"`
	Venue             string               `json:"venue"`
	PerformanceDate   time.Time            `json:"performanceDate"`
	TechnicalDirector string               `json:"technicalDirector"`
	Status            domain.SessionStatus `json:"status"`
	Version           uint64               `json:"version"`
	DeviceCount       int                  `json:"deviceCount"`
	CueCount          int                  `json:"cueCount"`
	PassedCount       int                  `json:"passedCount"`
	FailedCount       int                  `json:"failedCount"`
	CreatedAt         time.Time            `json:"createdAt"`
}

type TimelineEntry struct {
	Sequence uint64          `json:"sequence"`
	Version  uint64          `json:"version"`
	Type     string          `json:"type"`
	Label    string          `json:"label"`
	At       time.Time       `json:"at"`
	Checksum string          `json:"checksum"`
	Data     json.RawMessage `json:"data"`
}

type ViolationItem struct {
	CueID       string             `json:"cueID"`
	CueSequence int                `json:"cueSequence"`
	CueAction   string             `json:"cueAction"`
	AttemptID   string             `json:"attemptID"`
	Violations  []domain.Violation `json:"violations"`
}

type SessionDetail struct {
	Session          domain.ValidationSession     `json:"session"`
	Devices          []domain.RiggingDevice       `json:"devices"`
	Cues             []domain.SafetyCue           `json:"cues"`
	Attempts         []domain.CueAttempt          `json:"attempts"`
	Reviews          []domain.SafetyReview        `json:"reviews"`
	PendingCue       *domain.SafetyCue            `json:"pendingCue,omitempty"`
	Violations       []ViolationItem              `json:"violations"`
	Timeline         []TimelineEntry              `json:"timeline"`
	Certificate      *domain.ReadinessCertificate `json:"certificate,omitempty"`
	CertificateValid bool                         `json:"certificateValid"`
	CorrectionTasks  []domain.CorrectionTask      `json:"correctionTasks"`
}

type CommandResult struct {
	Commit journal.Commit `json:"commit"`
	Detail SessionDetail  `json:"detail"`
}

func summaryOf(aggregate *domain.Aggregate) SessionSummary {
	summary := SessionSummary{ID: aggregate.Session.ID, ProductionName: aggregate.Session.ProductionName, Venue: aggregate.Session.Venue, PerformanceDate: aggregate.Session.PerformanceDate, TechnicalDirector: aggregate.Session.TechnicalDirector, Status: aggregate.Session.Status, Version: aggregate.Session.Version, DeviceCount: len(aggregate.Devices), CueCount: len(aggregate.Cues), CreatedAt: aggregate.Session.CreatedAt}
	for _, cue := range aggregate.Cues {
		if cue.Status == domain.CuePassed {
			summary.PassedCount++
		}
		if cue.Status == domain.CueFailed {
			summary.FailedCount++
		}
	}
	return summary
}

func eventLabel(eventType string) string {
	labels := map[string]string{
		domain.EventSessionCreated: "创建验证会话", domain.EventDeviceAdded: "登记吊挂设备",
		domain.EventDeviceUpdated: "修订吊挂设备", domain.EventDeviceDeleted: "删除吊挂设备",
		domain.EventCueAdded: "配置安全动作", domain.EventConfigurationReady: "确认验证方案",
		domain.EventCueUpdated: "修订安全动作", domain.EventCueDeleted: "删除安全动作", domain.EventCuesReordered: "调整动作顺序",
		domain.EventRunStarted: "启动干运行", domain.EventAttemptRecorded: "录入动作实测",
		domain.EventReviewRequested: "进入安全复核", domain.EventReviewCompleted: "完成安全复核",
		domain.EventCorrectionSubmitted: "提交整改说明", domain.EventCertificateIssued: "签发就绪证书",
		domain.EventCorrectionTaskUpdated: "维护整改任务",
	}
	if label := labels[eventType]; label != "" {
		return label
	}
	return eventType
}

func detailOf(aggregate *domain.Aggregate, records []journal.Record) SessionDetail {
	detail := SessionDetail{Session: aggregate.Session, Devices: aggregate.OrderedDevices(), Cues: aggregate.OrderedCues(), Reviews: slices.Clone(aggregate.Reviews), PendingCue: aggregate.NextPendingCue(), Certificate: aggregate.Certificate}
	for _, cue := range detail.Cues {
		attempts := aggregate.Attempts[cue.ID]
		detail.Attempts = append(detail.Attempts, attempts...)
		if len(attempts) > 0 {
			latest := attempts[len(attempts)-1]
			if len(latest.Violations) > 0 {
				detail.Violations = append(detail.Violations, ViolationItem{CueID: cue.ID, CueSequence: cue.Sequence, CueAction: cue.Action, AttemptID: latest.ID, Violations: slices.Clone(latest.Violations)})
			}
		}
	}
	for cueID := range aggregate.CorrectionCueIDs {
		if task, exists := aggregate.CorrectionTasks[cueID]; exists {
			detail.CorrectionTasks = append(detail.CorrectionTasks, task)
		}
	}
	slices.SortFunc(detail.CorrectionTasks, func(left, right domain.CorrectionTask) int {
		return aggregate.Cues[left.CueID].Sequence - aggregate.Cues[right.CueID].Sequence
	})
	for _, record := range records {
		if record.Event.SessionID != aggregate.Session.ID {
			continue
		}
		detail.Timeline = append(detail.Timeline, TimelineEntry{Sequence: record.Sequence, Version: record.Event.Version, Type: record.Event.Type, Label: eventLabel(record.Event.Type), At: record.Event.At, Checksum: record.Checksum, Data: record.Event.Data})
	}
	if detail.Certificate != nil {
		detail.CertificateValid = domain.VerifyCertificate(*detail.Certificate)
	}
	return detail
}

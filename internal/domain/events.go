package domain

import (
	"encoding/json"
	"fmt"
	"time"
)

const (
	EventSessionCreated        = "session.created"
	EventDeviceAdded           = "device.added"
	EventDeviceUpdated         = "device.updated"
	EventDeviceDeleted         = "device.deleted"
	EventCueAdded              = "cue.added"
	EventCueUpdated            = "cue.updated"
	EventCueDeleted            = "cue.deleted"
	EventCuesReordered         = "cues.reordered"
	EventConfigurationReady    = "configuration.prepared"
	EventRunStarted            = "run.started"
	EventAttemptRecorded       = "attempt.recorded"
	EventReviewRequested       = "review.requested"
	EventReviewCompleted       = "review.completed"
	EventCorrectionSubmitted   = "correction.submitted"
	EventCorrectionTaskUpdated = "correction.task.updated"
	EventCertificateIssued     = "certificate.issued"
)

type Event struct {
	Type      string          `json:"type"`
	SessionID string          `json:"sessionID"`
	Version   uint64          `json:"version"`
	At        time.Time       `json:"at"`
	Data      json.RawMessage `json:"data"`
}

type SessionCreated struct {
	Session ValidationSession `json:"session"`
}

type DeviceAdded struct {
	Device RiggingDevice `json:"device"`
}

type DeviceUpdated struct {
	Before RiggingDevice `json:"before"`
	After  RiggingDevice `json:"after"`
}

type DeviceDeleted struct {
	Device RiggingDevice `json:"device"`
}

type CueAdded struct {
	Cue SafetyCue `json:"cue"`
}

type CueUpdated struct {
	Before SafetyCue `json:"before"`
	After  SafetyCue `json:"after"`
}

type CueDeleted struct {
	Cue SafetyCue `json:"cue"`
}

type CuesReordered struct {
	Before []string `json:"before"`
	After  []string `json:"after"`
}

type ConfigurationPrepared struct{}

type RunStarted struct {
	Correction bool `json:"correction"`
}

type AttemptRecorded struct {
	Attempt CueAttempt `json:"attempt"`
}

type ReviewRequested struct{}

type ReviewCompleted struct {
	Review          SafetyReview     `json:"review"`
	CorrectionTasks []CorrectionTask `json:"correctionTasks,omitempty"`
}

type CorrectionSubmitted struct {
	Note       string   `json:"note,omitempty"`
	TaskCueIDs []string `json:"taskCueIDs"`
}

type CorrectionTaskUpdated struct {
	Before CorrectionTask `json:"before"`
	After  CorrectionTask `json:"after"`
}

type CertificateIssued struct {
	Certificate ReadinessCertificate `json:"certificate"`
}

func MakeEvent(eventType, sessionID string, version uint64, at time.Time, payload any) (Event, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return Event{}, fmt.Errorf("encode domain event %s: %w", eventType, err)
	}
	return Event{Type: eventType, SessionID: sessionID, Version: version, At: at.UTC(), Data: data}, nil
}

func DecodeEvent[T any](event Event) (T, error) {
	var payload T
	if err := json.Unmarshal(event.Data, &payload); err != nil {
		return payload, fmt.Errorf("decode %s at version %d: %w", event.Type, event.Version, err)
	}
	return payload, nil
}

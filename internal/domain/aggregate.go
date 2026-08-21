package domain

import (
	"fmt"
	"slices"
	"time"
)

type Aggregate struct {
	Session          ValidationSession
	Devices          map[string]RiggingDevice
	Cues             map[string]SafetyCue
	Attempts         map[string][]CueAttempt
	Reviews          []SafetyReview
	Certificate      *ReadinessCertificate
	CorrectionCueIDs map[string]bool
	CorrectionTasks  map[string]CorrectionTask
}

func NewAggregate() *Aggregate {
	return &Aggregate{
		Devices:          make(map[string]RiggingDevice),
		Cues:             make(map[string]SafetyCue),
		Attempts:         make(map[string][]CueAttempt),
		CorrectionCueIDs: make(map[string]bool),
		CorrectionTasks:  make(map[string]CorrectionTask),
	}
}

func (a *Aggregate) Normalize() {
	if a.Devices == nil {
		a.Devices = make(map[string]RiggingDevice)
	}
	if a.Cues == nil {
		a.Cues = make(map[string]SafetyCue)
	}
	if a.Attempts == nil {
		a.Attempts = make(map[string][]CueAttempt)
	}
	if a.CorrectionCueIDs == nil {
		a.CorrectionCueIDs = make(map[string]bool)
	}
	if a.CorrectionTasks == nil {
		a.CorrectionTasks = make(map[string]CorrectionTask)
	}
	if a.Session.Status == SessionCorrection {
		for cueID := range a.CorrectionCueIDs {
			if _, exists := a.CorrectionTasks[cueID]; exists {
				continue
			}
			attempts := a.Attempts[cueID]
			if len(attempts) == 0 {
				continue
			}
			attempt := attempts[len(attempts)-1]
			a.CorrectionTasks[cueID] = CorrectionTask{CueID: cueID, AttemptID: attempt.ID, Violations: slices.Clone(attempt.Violations), UpdatedAt: attempt.RecordedAt}
		}
	}
}

func (a *Aggregate) Clone() *Aggregate {
	a.Normalize()
	copy := NewAggregate()
	copy.Session = a.Session
	copy.Session.DeviceIDs = slices.Clone(a.Session.DeviceIDs)
	copy.Session.CueIDs = slices.Clone(a.Session.CueIDs)
	for id, device := range a.Devices {
		copy.Devices[id] = device
	}
	for id, cue := range a.Cues {
		copy.Cues[id] = cue
	}
	for id, attempts := range a.Attempts {
		copy.Attempts[id] = slices.Clone(attempts)
	}
	copy.Reviews = slices.Clone(a.Reviews)
	for id, needed := range a.CorrectionCueIDs {
		copy.CorrectionCueIDs[id] = needed
	}
	for id, task := range a.CorrectionTasks {
		task.Violations = slices.Clone(task.Violations)
		if task.ClosedAt != nil {
			closed := *task.ClosedAt
			task.ClosedAt = &closed
		}
		copy.CorrectionTasks[id] = task
	}
	if a.Certificate != nil {
		certificate := *a.Certificate
		copy.Certificate = &certificate
	}
	if a.Session.CertifiedAt != nil {
		at := *a.Session.CertifiedAt
		copy.Session.CertifiedAt = &at
	}
	return copy
}

func (a *Aggregate) Apply(event Event) error {
	if event.Version != a.Session.Version+1 {
		return fmt.Errorf("apply %s: expected version %d, got %d", event.Type, a.Session.Version+1, event.Version)
	}
	switch event.Type {
	case EventSessionCreated:
		payload, err := DecodeEvent[SessionCreated](event)
		if err != nil {
			return err
		}
		a.Session = payload.Session
		a.Session.Version = event.Version
	case EventDeviceAdded:
		payload, err := DecodeEvent[DeviceAdded](event)
		if err != nil {
			return err
		}
		a.Devices[payload.Device.ID] = payload.Device
		a.Session.DeviceIDs = append(a.Session.DeviceIDs, payload.Device.ID)
	case EventDeviceUpdated:
		payload, err := DecodeEvent[DeviceUpdated](event)
		if err != nil {
			return err
		}
		a.Devices[payload.After.ID] = payload.After
	case EventDeviceDeleted:
		payload, err := DecodeEvent[DeviceDeleted](event)
		if err != nil {
			return err
		}
		delete(a.Devices, payload.Device.ID)
		a.Session.DeviceIDs = slices.DeleteFunc(a.Session.DeviceIDs, func(id string) bool { return id == payload.Device.ID })
	case EventCueAdded:
		payload, err := DecodeEvent[CueAdded](event)
		if err != nil {
			return err
		}
		a.Cues[payload.Cue.ID] = payload.Cue
		a.Session.CueIDs = append(a.Session.CueIDs, payload.Cue.ID)
	case EventCueUpdated:
		payload, err := DecodeEvent[CueUpdated](event)
		if err != nil {
			return err
		}
		a.Cues[payload.After.ID] = payload.After
	case EventCueDeleted:
		payload, err := DecodeEvent[CueDeleted](event)
		if err != nil {
			return err
		}
		delete(a.Cues, payload.Cue.ID)
		delete(a.Attempts, payload.Cue.ID)
		a.Session.CueIDs = slices.DeleteFunc(a.Session.CueIDs, func(id string) bool { return id == payload.Cue.ID })
		for id, cue := range a.Cues {
			if cue.Sequence > payload.Cue.Sequence {
				cue.Sequence--
				a.Cues[id] = cue
			}
		}
	case EventCuesReordered:
		payload, err := DecodeEvent[CuesReordered](event)
		if err != nil {
			return err
		}
		for index, id := range payload.After {
			cue := a.Cues[id]
			cue.Sequence = index + 1
			a.Cues[id] = cue
		}
		a.Session.CueIDs = slices.Clone(payload.After)
	case EventConfigurationReady:
		a.Session.Status = SessionPrepared
	case EventRunStarted:
		a.Session.Status = SessionRunning
	case EventAttemptRecorded:
		payload, err := DecodeEvent[AttemptRecorded](event)
		if err != nil {
			return err
		}
		a.Attempts[payload.Attempt.CueID] = append(a.Attempts[payload.Attempt.CueID], payload.Attempt)
		cue := a.Cues[payload.Attempt.CueID]
		cue.AttemptCount = payload.Attempt.AttemptNo
		if payload.Attempt.Result == AttemptPass {
			cue.Status = CuePassed
		} else {
			cue.Status = CueFailed
		}
		a.Cues[cue.ID] = cue
		delete(a.CorrectionCueIDs, cue.ID)
	case EventReviewRequested:
		a.Session.Status = SessionReview
	case EventReviewCompleted:
		payload, err := DecodeEvent[ReviewCompleted](event)
		if err != nil {
			return err
		}
		a.Reviews = append(a.Reviews, payload.Review)
		if payload.Review.Decision == ReviewNeedsCorrection {
			a.Session.Status = SessionCorrection
			clear(a.CorrectionCueIDs)
			for id, cue := range a.Cues {
				if cue.Status == CueFailed {
					a.CorrectionCueIDs[id] = true
				}
			}
			if a.CorrectionTasks == nil {
				a.CorrectionTasks = make(map[string]CorrectionTask)
			}
			clear(a.CorrectionTasks)
			for _, task := range payload.CorrectionTasks {
				a.CorrectionTasks[task.CueID] = task
			}
		}
	case EventCorrectionTaskUpdated:
		payload, err := DecodeEvent[CorrectionTaskUpdated](event)
		if err != nil {
			return err
		}
		if a.CorrectionTasks == nil {
			a.CorrectionTasks = make(map[string]CorrectionTask)
		}
		a.CorrectionTasks[payload.After.CueID] = payload.After
	case EventCorrectionSubmitted:
		payload, err := DecodeEvent[CorrectionSubmitted](event)
		if err != nil {
			return err
		}
		ids := payload.TaskCueIDs
		if len(ids) == 0 {
			for id := range a.CorrectionCueIDs {
				ids = append(ids, id)
			}
		}
		for _, id := range ids {
			cue := a.Cues[id]
			cue.Status = CuePending
			a.Cues[id] = cue
		}
	case EventCertificateIssued:
		payload, err := DecodeEvent[CertificateIssued](event)
		if err != nil {
			return err
		}
		a.Certificate = &payload.Certificate
		a.Session.Status = SessionCertified
		issued := payload.Certificate.IssuedAt
		a.Session.CertifiedAt = &issued
	default:
		return fmt.Errorf("unknown domain event %q", event.Type)
	}
	if event.Type != EventSessionCreated {
		a.Session.Version = event.Version
	}
	return nil
}

func (a *Aggregate) emit(eventType string, at time.Time, payload any) (Event, error) {
	event, err := MakeEvent(eventType, a.Session.ID, a.Session.Version+1, at, payload)
	if err != nil {
		return Event{}, err
	}
	if err := a.Apply(event); err != nil {
		return Event{}, err
	}
	return event, nil
}

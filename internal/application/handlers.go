package application

import (
	"context"

	"stageready/internal/domain"
)

func (s *Service) CreateSession(command CreateSessionCommand) (CommandResult, error) {
	return s.createSession(context.Background(), command)
}

func (s *Service) createSession(ctx context.Context, command CreateSessionCommand) (CommandResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := validateCommand(command.ExpectedVersion, command.IdempotencyKey); err != nil {
		return CommandResult{}, err
	}
	if prior, found, err := s.priorAnyResult(command.IdempotencyKey); found || err != nil {
		return prior, err
	}
	if command.ID == "" {
		command.ID = NewID("ses")
	}
	if prior, found, err := s.priorResult(command.ID, command.IdempotencyKey); found || err != nil {
		return prior, err
	}
	if _, exists := s.sessions[command.ID]; exists {
		return CommandResult{}, &RequestError{Code: "SESSION_EXISTS", Message: "会话 ID 已存在"}
	}
	if command.ExpectedVersion != 0 {
		return CommandResult{}, &RequestError{Code: "INVALID_INITIAL_VERSION", Message: "创建会话的 expectedVersion 必须为 0"}
	}
	draft, event, err := domain.CreateSession(domain.CreateSessionInput{ID: command.ID, ProductionName: command.ProductionName, Venue: command.Venue, PerformanceDate: command.PerformanceDate, TechnicalDirector: command.TechnicalDirector}, s.clock())
	if err != nil {
		return CommandResult{}, err
	}
	if ctx != nil {
		select {
		case <-ctx.Done():
			return CommandResult{}, &CanceledError{Cause: ctx.Err()}
		default:
		}
	}
	commit, err := s.journal.Append(command.ID, 0, command.IdempotencyKey, []domain.Event{event})
	if err != nil {
		return CommandResult{}, err
	}
	return s.publish(command.ID, draft, commit), nil
}

func (s *Service) AddDevice(command AddDeviceCommand) (CommandResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := validateCommand(command.ExpectedVersion, command.IdempotencyKey); err != nil {
		return CommandResult{}, err
	}
	if prior, found, err := s.priorResult(command.SessionID, command.IdempotencyKey); found || err != nil {
		return prior, err
	}
	draft, err := s.aggregateCopy(command.SessionID)
	if err != nil {
		return CommandResult{}, err
	}
	if command.ID == "" {
		command.ID = NewID("dev")
	}
	event, err := draft.AddDevice(domain.RiggingDevice{ID: command.ID, Name: command.Name, DeviceType: command.DeviceType, RatedLoadKg: command.RatedLoadKg, SafeZone: command.SafeZone, EmergencyStopRequired: command.EmergencyStopRequired}, s.clock())
	if err != nil {
		return CommandResult{}, err
	}
	commit, err := s.journal.Append(command.SessionID, command.ExpectedVersion, command.IdempotencyKey, []domain.Event{event})
	if err != nil {
		return CommandResult{}, err
	}
	return s.publish(command.SessionID, draft, commit), nil
}

func (s *Service) AddCue(command AddCueCommand) (CommandResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := validateCommand(command.ExpectedVersion, command.IdempotencyKey); err != nil {
		return CommandResult{}, err
	}
	if prior, found, err := s.priorResult(command.SessionID, command.IdempotencyKey); found || err != nil {
		return prior, err
	}
	draft, err := s.aggregateCopy(command.SessionID)
	if err != nil {
		return CommandResult{}, err
	}
	if command.ID == "" {
		command.ID = NewID("cue")
	}
	event, err := draft.AddCue(domain.SafetyCue{ID: command.ID, Sequence: command.Sequence, DeviceID: command.DeviceID, Action: command.Action, ExpectedLoadKg: command.ExpectedLoadKg, MinimumClearanceCm: command.MinimumClearanceCm, MaximumStopMs: command.MaximumStopMs}, s.clock())
	if err != nil {
		return CommandResult{}, err
	}
	commit, err := s.journal.Append(command.SessionID, command.ExpectedVersion, command.IdempotencyKey, []domain.Event{event})
	if err != nil {
		return CommandResult{}, err
	}
	return s.publish(command.SessionID, draft, commit), nil
}

func (s *Service) UpdateDevice(command UpdateDeviceCommand) (CommandResult, error) {
	return s.change(command.SessionID, command.CommandMeta, func(draft *domain.Aggregate) ([]domain.Event, error) {
		event, err := draft.UpdateDevice(domain.RiggingDevice{ID: command.ID, Name: command.Name, DeviceType: command.DeviceType, RatedLoadKg: command.RatedLoadKg, SafeZone: command.SafeZone, EmergencyStopRequired: command.EmergencyStopRequired}, s.clock())
		return single(event, err)
	})
}

func (s *Service) DeleteDevice(command DeleteDeviceCommand) (CommandResult, error) {
	return s.change(command.SessionID, command.CommandMeta, func(draft *domain.Aggregate) ([]domain.Event, error) {
		event, err := draft.DeleteDevice(command.DeviceID, s.clock())
		return single(event, err)
	})
}

func (s *Service) UpdateCue(command UpdateCueCommand) (CommandResult, error) {
	return s.change(command.SessionID, command.CommandMeta, func(draft *domain.Aggregate) ([]domain.Event, error) {
		return draft.UpdateCue(domain.SafetyCue{ID: command.ID, Sequence: command.Sequence, DeviceID: command.DeviceID, Action: command.Action, ExpectedLoadKg: command.ExpectedLoadKg, MinimumClearanceCm: command.MinimumClearanceCm, MaximumStopMs: command.MaximumStopMs}, s.clock())
	})
}

func (s *Service) DeleteCue(command DeleteCueCommand) (CommandResult, error) {
	return s.change(command.SessionID, command.CommandMeta, func(draft *domain.Aggregate) ([]domain.Event, error) {
		event, err := draft.DeleteCue(command.CueID, s.clock())
		return single(event, err)
	})
}

func (s *Service) ReorderCues(command ReorderCuesCommand) (CommandResult, error) {
	return s.change(command.SessionID, command.CommandMeta, func(draft *domain.Aggregate) ([]domain.Event, error) {
		event, err := draft.ReorderCues(command.CueIDs, s.clock())
		return single(event, err)
	})
}

func (s *Service) PreflightConfiguration(sessionID string, input ConfigurationPreflightInput) (domain.ConfigurationPreflight, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	aggregate := s.sessions[sessionID]
	if aggregate == nil {
		return domain.ConfigurationPreflight{}, &NotFoundError{Resource: "session", ID: sessionID}
	}
	return aggregate.PreflightConfiguration(domain.BatchConfigurationInput{Devices: input.Devices, Cues: input.Cues}), nil
}

func (s *Service) ConfirmConfigurationBatch(command ConfigurationBatchCommand) (CommandResult, error) {
	return s.change(command.SessionID, command.CommandMeta, func(draft *domain.Aggregate) ([]domain.Event, error) {
		events, _, err := draft.ConfirmConfigurationBatch(domain.BatchConfigurationInput{Devices: command.Devices, Cues: command.Cues}, s.clock())
		return events, err
	})
}

func single(event domain.Event, err error) ([]domain.Event, error) {
	if err != nil {
		return nil, err
	}
	return []domain.Event{event}, nil
}

func (s *Service) change(sessionID string, meta CommandMeta, mutation func(*domain.Aggregate) ([]domain.Event, error)) (CommandResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := validateCommand(meta.ExpectedVersion, meta.IdempotencyKey); err != nil {
		return CommandResult{}, err
	}
	if prior, found, err := s.priorResult(sessionID, meta.IdempotencyKey); found || err != nil {
		return prior, err
	}
	draft, err := s.aggregateCopy(sessionID)
	if err != nil {
		return CommandResult{}, err
	}
	events, err := mutation(draft)
	if err != nil {
		return CommandResult{}, err
	}
	commit, err := s.journal.Append(sessionID, meta.ExpectedVersion, meta.IdempotencyKey, events)
	if err != nil {
		return CommandResult{}, err
	}
	return s.publish(sessionID, draft, commit), nil
}

func (s *Service) Prepare(command SessionCommand) (CommandResult, error) {
	return s.singleEvent(command, func(a *domain.Aggregate) (domain.Event, error) { return a.Prepare(s.clock()) })
}
func (s *Service) StartRun(command SessionCommand) (CommandResult, error) {
	return s.singleEvent(command, func(a *domain.Aggregate) (domain.Event, error) { return a.StartRun(s.clock()) })
}

func (s *Service) singleEvent(command SessionCommand, change func(*domain.Aggregate) (domain.Event, error)) (CommandResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := validateCommand(command.ExpectedVersion, command.IdempotencyKey); err != nil {
		return CommandResult{}, err
	}
	if prior, found, err := s.priorResult(command.SessionID, command.IdempotencyKey); found || err != nil {
		return prior, err
	}
	draft, err := s.aggregateCopy(command.SessionID)
	if err != nil {
		return CommandResult{}, err
	}
	event, err := change(draft)
	if err != nil {
		return CommandResult{}, err
	}
	commit, err := s.journal.Append(command.SessionID, command.ExpectedVersion, command.IdempotencyKey, []domain.Event{event})
	if err != nil {
		return CommandResult{}, err
	}
	return s.publish(command.SessionID, draft, commit), nil
}

package application

import "stageready/internal/domain"

func (s *Service) RecordAttempt(command RecordAttemptCommand) (CommandResult, error) {
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
		command.ID = NewID("att")
	}
	events, err := draft.RecordAttempt(domain.RecordAttemptInput{ID: command.ID, CueID: command.CueID, MeasuredLoadKg: command.MeasuredLoadKg, MeasuredClearanceCm: command.MeasuredClearanceCm, MeasuredStopMs: command.MeasuredStopMs, Operator: command.Operator, EvidenceNote: command.EvidenceNote}, s.clock())
	if err != nil {
		return CommandResult{}, err
	}
	commit, err := s.journal.Append(command.SessionID, command.ExpectedVersion, command.IdempotencyKey, events)
	if err != nil {
		return CommandResult{}, err
	}
	return s.publish(command.SessionID, draft, commit), nil
}

func (s *Service) RecordAttemptBatch(command RecordAttemptBatchCommand) (CommandResult, error) {
	return s.change(command.SessionID, command.CommandMeta, func(draft *domain.Aggregate) ([]domain.Event, error) {
		inputs := make([]domain.RecordAttemptInput, len(command.Attempts))
		for index, item := range command.Attempts {
			id := item.ID
			if id == "" {
				id = NewID("att")
			}
			inputs[index] = domain.RecordAttemptInput{ID: id, CueID: item.CueID, MeasuredLoadKg: item.MeasuredLoadKg, MeasuredClearanceCm: item.MeasuredClearanceCm, MeasuredStopMs: item.MeasuredStopMs, Operator: item.Operator, EvidenceNote: item.EvidenceNote}
		}
		return draft.RecordAttemptBatch(inputs, s.clock())
	})
}

func (s *Service) CompleteReview(command CompleteReviewCommand) (CommandResult, error) {
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
		command.ID = NewID("rev")
	}
	event, err := draft.CompleteReview(domain.CompleteReviewInput{ID: command.ID, Reviewer: command.Reviewer, Decision: command.Decision, Findings: command.Findings, CorrectionNote: command.CorrectionNote}, s.clock())
	if err != nil {
		return CommandResult{}, err
	}
	commit, err := s.journal.Append(command.SessionID, command.ExpectedVersion, command.IdempotencyKey, []domain.Event{event})
	if err != nil {
		return CommandResult{}, err
	}
	return s.publish(command.SessionID, draft, commit), nil
}

func (s *Service) SubmitCorrection(command SubmitCorrectionCommand) (CommandResult, error) {
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
	events, err := draft.SubmitCorrection(command.Note, s.clock())
	if err != nil {
		return CommandResult{}, err
	}
	commit, err := s.journal.Append(command.SessionID, command.ExpectedVersion, command.IdempotencyKey, events)
	if err != nil {
		return CommandResult{}, err
	}
	return s.publish(command.SessionID, draft, commit), nil
}

func (s *Service) UpdateCorrectionTask(command UpdateCorrectionTaskCommand) (CommandResult, error) {
	return s.change(command.SessionID, command.CommandMeta, func(draft *domain.Aggregate) ([]domain.Event, error) {
		event, err := draft.UpdateCorrectionTask(domain.UpdateCorrectionTaskInput{CueID: command.CueID, Measure: command.Measure, Owner: command.Owner, EvidenceNote: command.EvidenceNote, Closed: command.Closed}, s.clock())
		return single(event, err)
	})
}

func (s *Service) IssueCertificate(command IssueCertificateCommand) (CommandResult, error) {
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
		command.ID = NewID("cert")
	}
	event, err := draft.IssueCertificate(command.ID, s.journal.HeadHash(), s.clock())
	if err != nil {
		return CommandResult{}, err
	}
	commit, err := s.journal.Append(command.SessionID, command.ExpectedVersion, command.IdempotencyKey, []domain.Event{event})
	if err != nil {
		return CommandResult{}, err
	}
	return s.publish(command.SessionID, draft, commit), nil
}

package application

import (
	"context"
	"errors"
	"os"
	"slices"
	"sync"
	"time"

	"stageready/internal/domain"
	"stageready/internal/journal"
)

type Clock func() time.Time

type Service struct {
	mu       sync.RWMutex
	journal  *journal.Store
	sessions map[string]*domain.Aggregate
	clock    Clock
}

type snapshotState struct {
	Sessions map[string]*domain.Aggregate `json:"sessions"`
}

func NewService(store *journal.Store, clock Clock) (*Service, error) {
	if clock == nil {
		clock = time.Now
	}
	service := &Service{journal: store, sessions: make(map[string]*domain.Aggregate), clock: clock}
	var snapshot snapshotState
	if err := store.LoadSnapshot(&snapshot); err == nil && snapshot.Sessions != nil {
		service.sessions = snapshot.Sessions
		for _, aggregate := range service.sessions {
			aggregate.Normalize()
		}
		return service, nil
	}
	if err := service.rebuild(store.Events()); err != nil {
		return nil, err
	}
	_ = service.saveSnapshot()
	return service, nil
}

func (s *Service) rebuild(events []domain.Event) error {
	for _, event := range events {
		aggregate := s.sessions[event.SessionID]
		if aggregate == nil {
			aggregate = domain.NewAggregate()
			s.sessions[event.SessionID] = aggregate
		}
		if err := aggregate.Apply(event); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) saveSnapshot() error {
	return s.journal.SaveSnapshot(snapshotState{Sessions: s.sessions})
}

func (s *Service) ListSessions() []SessionSummary {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]SessionSummary, 0, len(s.sessions))
	for _, aggregate := range s.sessions {
		result = append(result, summaryOf(aggregate))
	}
	slices.SortFunc(result, func(left, right SessionSummary) int { return right.CreatedAt.Compare(left.CreatedAt) })
	return result
}

func (s *Service) GetSession(id string) (SessionDetail, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	aggregate := s.sessions[id]
	if aggregate == nil {
		return SessionDetail{}, &NotFoundError{Resource: "session", ID: id}
	}
	return detailOf(aggregate, s.journal.Records()), nil
}

func (s *Service) priorResult(sessionID, key string) (CommandResult, bool, error) {
	commit, exists := s.journal.LookupCommit(key)
	if !exists {
		return CommandResult{}, false, nil
	}
	if commit.SessionID != sessionID {
		return CommandResult{}, false, &journal.IdempotencyError{Key: key, SessionID: commit.SessionID}
	}
	commit.Duplicate = true
	result, err := s.resultAtCommit(commit)
	return result, true, err
}

func (s *Service) priorAnyResult(key string) (CommandResult, bool, error) {
	commit, exists := s.journal.LookupCommit(key)
	if !exists {
		return CommandResult{}, false, nil
	}
	commit.Duplicate = true
	result, err := s.resultAtCommit(commit)
	return result, true, err
}

func (s *Service) resultAtCommit(commit journal.Commit) (CommandResult, error) {
	records := s.journal.Records()
	if commit.Sequence > uint64(len(records)) {
		return CommandResult{}, &RequestError{Code: "IDEMPOTENCY_RESULT_UNAVAILABLE", Message: "原提交结果超出当前事件日志范围"}
	}
	aggregate := domain.NewAggregate()
	for _, record := range records[:commit.Sequence] {
		if record.Event.SessionID != commit.SessionID || record.Event.Version > commit.ToVersion {
			continue
		}
		if err := aggregate.Apply(record.Event); err != nil {
			return CommandResult{}, err
		}
	}
	if aggregate.Session.ID == "" {
		return CommandResult{}, &NotFoundError{Resource: "session", ID: commit.SessionID}
	}
	return CommandResult{Commit: commit, Detail: detailOf(aggregate, records[:commit.Sequence])}, nil
}

func (s *Service) aggregateCopy(id string) (*domain.Aggregate, error) {
	aggregate := s.sessions[id]
	if aggregate == nil {
		return nil, &NotFoundError{Resource: "session", ID: id}
	}
	return aggregate.Clone(), nil
}

func (s *Service) publish(id string, draft *domain.Aggregate, commit journal.Commit) CommandResult {
	s.sessions[id] = draft
	_ = s.saveSnapshot()
	return CommandResult{Commit: commit, Detail: detailOf(draft, s.journal.Records())}
}

// CreateSessionContext adapts request cancellation to the create workflow.
// If the context is already canceled, no event is persisted and no in-memory
// state is published. Once the journal commit and publish succeed, the result
// is returned as-is so callers never observe a cancellation error for a session
// that actually was committed.
func (s *Service) CreateSessionContext(ctx context.Context, command CreateSessionCommand) (CommandResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	return s.createSession(ctx, command)
}

func (s *Service) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	snapshotErr := s.saveSnapshot()
	closeErr := s.journal.Close()
	return errors.Join(snapshotErr, closeErr)
}

func IsSnapshotMissing(err error) bool { return errors.Is(err, os.ErrNotExist) }

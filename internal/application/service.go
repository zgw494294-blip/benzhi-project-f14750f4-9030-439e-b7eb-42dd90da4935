package application

import (
	"errors"
	"log/slog"
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
		// 隔离校验和有效但投影载荷损坏的条目（例如聚合反序列化为 null）。
		// 丢弃这些条目后，仅当事件日志中存在对应事件时才重放恢复，否则查询返回 NotFound。
		corrupted := false
		normalized := make(map[string]*domain.Aggregate, len(snapshot.Sessions))
		for id, aggregate := range snapshot.Sessions {
			if aggregate == nil {
				corrupted = true
				slog.Warn("丢弃损坏的会话投影，将从事件日志恢复", "sessionID", id)
				continue
			}
			aggregate.Normalize()
			normalized[id] = aggregate
		}
		service.sessions = normalized
		if corrupted {
			// 快照中存在不可用投影，重放事件日志重建丢失的会话并覆盖快照。
			if err := service.rebuild(store.Events()); err != nil {
				return nil, err
			}
			_ = service.saveSnapshot()
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

func (s *Service) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	snapshotErr := s.saveSnapshot()
	closeErr := s.journal.Close()
	return errors.Join(snapshotErr, closeErr)
}

func IsSnapshotMissing(err error) bool { return errors.Is(err, os.ErrNotExist) }

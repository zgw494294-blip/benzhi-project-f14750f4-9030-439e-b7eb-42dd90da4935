package journal

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sync"

	"stageready/internal/domain"
)

type Commit struct {
	SessionID   string `json:"sessionID"`
	FromVersion uint64 `json:"fromVersion"`
	ToVersion   uint64 `json:"toVersion"`
	Sequence    uint64 `json:"sequence"`
	HeadHash    string `json:"headHash"`
	Duplicate   bool   `json:"duplicate"`
}

type Store struct {
	mu              sync.Mutex
	dir             string
	path            string
	file            *os.File
	records         []Record
	sessionVersions map[string]uint64
	idempotency     map[string]Commit
	headHash        string
}

func Open(dir string) (*Store, error) {
	if dir == "" {
		return nil, errors.New("journal directory is required")
	}
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return nil, fmt.Errorf("create journal directory: %w", err)
	}
	path := filepath.Join(dir, "events.jsonl")
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o640)
	if err != nil {
		return nil, fmt.Errorf("open event journal: %w", err)
	}
	store := &Store{dir: dir, path: path, file: file, sessionVersions: make(map[string]uint64), idempotency: make(map[string]Commit)}
	if err := store.replay(); err != nil {
		file.Close()
		return nil, err
	}
	if _, err := file.Seek(0, 2); err != nil {
		file.Close()
		return nil, fmt.Errorf("seek event journal: %w", err)
	}
	return store, nil
}

func (s *Store) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.file == nil {
		return nil
	}
	err := s.file.Close()
	s.file = nil
	return err
}

func (s *Store) Append(sessionID string, expectedVersion uint64, idempotencyKey string, events []domain.Event) (Commit, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.file == nil {
		return Commit{}, errors.New("event journal is closed")
	}
	if idempotencyKey == "" {
		return Commit{}, errors.New("idempotency key is required")
	}
	if prior, exists := s.idempotency[idempotencyKey]; exists {
		if prior.SessionID != sessionID {
			return Commit{}, &IdempotencyError{Key: idempotencyKey, SessionID: prior.SessionID}
		}
		prior.Duplicate = true
		return prior, nil
	}
	actual := s.sessionVersions[sessionID]
	if actual != expectedVersion {
		return Commit{}, &ConflictError{SessionID: sessionID, Expected: expectedVersion, Actual: actual}
	}
	if len(events) == 0 {
		return Commit{}, errors.New("at least one event is required")
	}
	for index, event := range events {
		expected := expectedVersion + uint64(index) + 1
		if event.SessionID != sessionID || event.Version != expected {
			return Commit{}, fmt.Errorf("invalid event %d: session/version mismatch", index)
		}
	}
	offset, err := s.file.Seek(0, 1)
	if err != nil {
		return Commit{}, fmt.Errorf("read journal offset: %w", err)
	}
	previous := s.headHash
	newRecords := make([]Record, 0, len(events))
	encoder := json.NewEncoder(s.file)
	for _, event := range events {
		record := Record{Sequence: uint64(len(s.records) + len(newRecords) + 1), PreviousHash: previous, IdempotencyKey: idempotencyKey, Event: event}
		record.Checksum, err = checksum(record)
		if err != nil {
			s.rollback(offset)
			return Commit{}, fmt.Errorf("checksum event: %w", err)
		}
		if err := encoder.Encode(record); err != nil {
			s.rollback(offset)
			return Commit{}, fmt.Errorf("append event: %w", err)
		}
		previous = record.Checksum
		newRecords = append(newRecords, record)
	}
	if err := s.file.Sync(); err != nil {
		s.rollback(offset)
		return Commit{}, fmt.Errorf("sync event journal: %w", err)
	}
	s.records = append(s.records, newRecords...)
	s.headHash = previous
	toVersion := events[len(events)-1].Version
	s.sessionVersions[sessionID] = toVersion
	commit := Commit{SessionID: sessionID, FromVersion: expectedVersion + 1, ToVersion: toVersion, Sequence: uint64(len(s.records)), HeadHash: s.headHash}
	s.idempotency[idempotencyKey] = commit
	return commit, nil
}

func (s *Store) rollback(offset int64) {
	_ = s.file.Truncate(offset)
	_, _ = s.file.Seek(offset, 0)
}

func (s *Store) Records() []Record {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.records)
}

func (s *Store) Events() []domain.Event {
	s.mu.Lock()
	defer s.mu.Unlock()
	events := make([]domain.Event, 0, len(s.records))
	for _, record := range s.records {
		events = append(events, record.Event)
	}
	return events
}

func (s *Store) HeadHash() string { s.mu.Lock(); defer s.mu.Unlock(); return s.headHash }
func (s *Store) Sequence() uint64 { s.mu.Lock(); defer s.mu.Unlock(); return uint64(len(s.records)) }
func (s *Store) SessionVersion(id string) uint64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.sessionVersions[id]
}

// SessionIDs returns the IDs of all sessions that appear in the event journal.
func (s *Store) SessionIDs() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	ids := make([]string, 0, len(s.sessionVersions))
	for id := range s.sessionVersions {
		ids = append(ids, id)
	}
	return ids
}

func (s *Store) LookupCommit(idempotencyKey string) (Commit, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	commit, exists := s.idempotency[idempotencyKey]
	return commit, exists
}

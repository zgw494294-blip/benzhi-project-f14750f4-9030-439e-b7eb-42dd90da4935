package journal

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

func (s *Store) replay() error {
	if _, err := s.file.Seek(0, 0); err != nil {
		return fmt.Errorf("seek journal for replay: %w", err)
	}
	scanner := bufio.NewScanner(s.file)
	scanner.Buffer(make([]byte, 64*1024), 4*1024*1024)
	previous := ""
	line := 0
	for scanner.Scan() {
		line++
		var record Record
		if err := json.Unmarshal(scanner.Bytes(), &record); err != nil {
			return &CorruptionError{Line: line, Reason: "invalid JSON: " + err.Error()}
		}
		if err := validateRecord(record, uint64(line), previous); err != nil {
			return err
		}
		current := s.sessionVersions[record.Event.SessionID]
		if record.Event.Version != current+1 {
			return &CorruptionError{Line: line, Reason: "session version is not continuous"}
		}
		if prior, exists := s.idempotency[record.IdempotencyKey]; exists && prior.SessionID != record.Event.SessionID {
			return &CorruptionError{Line: line, Reason: "idempotency key reused across sessions"}
		}
		s.records = append(s.records, record)
		s.sessionVersions[record.Event.SessionID] = record.Event.Version
		commit := s.idempotency[record.IdempotencyKey]
		if commit.SessionID == "" {
			commit = Commit{SessionID: record.Event.SessionID, FromVersion: record.Event.Version}
		}
		commit.ToVersion = record.Event.Version
		commit.Sequence = record.Sequence
		commit.HeadHash = record.Checksum
		s.idempotency[record.IdempotencyKey] = commit
		previous = record.Checksum
	}
	if err := scanner.Err(); err != nil {
		if errors.Is(err, io.ErrUnexpectedEOF) {
			return &CorruptionError{Line: line + 1, Reason: "truncated JSON record"}
		}
		return fmt.Errorf("scan event journal: %w", err)
	}
	s.headHash = previous
	return nil
}

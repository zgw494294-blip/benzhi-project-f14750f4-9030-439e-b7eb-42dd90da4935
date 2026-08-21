package journal

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

var ErrSnapshotStale = errors.New("projection snapshot does not match event journal head")

type snapshotFile struct {
	Sequence uint64          `json:"sequence"`
	HeadHash string          `json:"headHash"`
	Payload  json.RawMessage `json:"payload"`
	Checksum string          `json:"checksum"`
}

func snapshotChecksum(sequence uint64, head string, payload json.RawMessage) string {
	encoded, _ := json.Marshal(struct {
		Sequence uint64          `json:"sequence"`
		HeadHash string          `json:"headHash"`
		Payload  json.RawMessage `json:"payload"`
	}{sequence, head, payload})
	hash := sha256.Sum256(encoded)
	return hex.EncodeToString(hash[:])
}

func (s *Store) SaveSnapshot(value any) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	payload, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("encode projection snapshot: %w", err)
	}
	snapshot := snapshotFile{Sequence: uint64(len(s.records)), HeadHash: s.headHash, Payload: payload}
	snapshot.Checksum = snapshotChecksum(snapshot.Sequence, snapshot.HeadHash, snapshot.Payload)
	encoded, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return fmt.Errorf("encode snapshot envelope: %w", err)
	}
	temporary, err := os.CreateTemp(s.dir, "snapshot-*.tmp")
	if err != nil {
		return fmt.Errorf("create temporary snapshot: %w", err)
	}
	temporaryName := temporary.Name()
	defer os.Remove(temporaryName)
	if err := temporary.Chmod(0o640); err != nil {
		temporary.Close()
		return err
	}
	if _, err := temporary.Write(encoded); err != nil {
		temporary.Close()
		return fmt.Errorf("write projection snapshot: %w", err)
	}
	if err := temporary.Sync(); err != nil {
		temporary.Close()
		return fmt.Errorf("sync projection snapshot: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	if err := os.Rename(temporaryName, filepath.Join(s.dir, "snapshot.json")); err != nil {
		return fmt.Errorf("publish projection snapshot: %w", err)
	}
	return nil
}

func (s *Store) LoadSnapshot(target any) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	encoded, err := os.ReadFile(filepath.Join(s.dir, "snapshot.json"))
	if err != nil {
		return err
	}
	var snapshot snapshotFile
	if err := json.Unmarshal(encoded, &snapshot); err != nil {
		return fmt.Errorf("decode projection snapshot: %w", err)
	}
	if snapshot.Checksum != snapshotChecksum(snapshot.Sequence, snapshot.HeadHash, snapshot.Payload) {
		return errors.New("projection snapshot checksum mismatch")
	}
	if snapshot.Sequence != uint64(len(s.records)) || snapshot.HeadHash != s.headHash {
		return ErrSnapshotStale
	}
	if err := json.Unmarshal(snapshot.Payload, target); err != nil {
		return fmt.Errorf("decode projection snapshot payload: %w", err)
	}
	return nil
}

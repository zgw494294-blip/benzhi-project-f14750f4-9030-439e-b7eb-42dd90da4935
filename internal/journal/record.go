package journal

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"

	"stageready/internal/domain"
)

type Record struct {
	Sequence       uint64       `json:"sequence"`
	PreviousHash   string       `json:"previousHash"`
	Checksum       string       `json:"checksum"`
	IdempotencyKey string       `json:"idempotencyKey"`
	Event          domain.Event `json:"event"`
}

type recordDigest struct {
	Sequence       uint64       `json:"sequence"`
	PreviousHash   string       `json:"previousHash"`
	IdempotencyKey string       `json:"idempotencyKey"`
	Event          domain.Event `json:"event"`
}

func checksum(record Record) (string, error) {
	encoded, err := json.Marshal(recordDigest{
		Sequence: record.Sequence, PreviousHash: record.PreviousHash,
		IdempotencyKey: record.IdempotencyKey, Event: record.Event,
	})
	if err != nil {
		return "", err
	}
	hash := sha256.Sum256(encoded)
	return hex.EncodeToString(hash[:]), nil
}

func validateRecord(record Record, expectedSequence uint64, expectedPrevious string) error {
	if record.Sequence != expectedSequence {
		return &CorruptionError{Line: int(expectedSequence), Reason: "sequence is not continuous"}
	}
	if record.PreviousHash != expectedPrevious {
		return &CorruptionError{Line: int(expectedSequence), Reason: "previousHash does not match prior checksum"}
	}
	expected, err := checksum(record)
	if err != nil {
		return err
	}
	if record.Checksum != expected {
		return &CorruptionError{Line: int(expectedSequence), Reason: "checksum mismatch"}
	}
	return nil
}

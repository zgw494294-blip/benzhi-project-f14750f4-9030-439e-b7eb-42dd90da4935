package journal

import "fmt"

type ConflictError struct {
	SessionID string
	Expected  uint64
	Actual    uint64
}

func (e *ConflictError) Error() string {
	return fmt.Sprintf("session %s version conflict: expected %d, actual %d", e.SessionID, e.Expected, e.Actual)
}

type CorruptionError struct {
	Line   int
	Reason string
}

func (e *CorruptionError) Error() string {
	return fmt.Sprintf("event journal corruption at line %d: %s", e.Line, e.Reason)
}

type IdempotencyError struct {
	Key       string
	SessionID string
}

func (e *IdempotencyError) Error() string {
	return fmt.Sprintf("idempotency key %q belongs to another command or session %s", e.Key, e.SessionID)
}

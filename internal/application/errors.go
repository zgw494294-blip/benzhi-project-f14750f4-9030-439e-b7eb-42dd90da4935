package application

import (
	"fmt"
)

type NotFoundError struct {
	Resource string
	ID       string
}

func (e *NotFoundError) Error() string { return fmt.Sprintf("%s %s not found", e.Resource, e.ID) }

type RequestError struct {
	Code    string
	Message string
	Field   string
}

func (e *RequestError) Error() string { return e.Message }

// CanceledError signals that a command was aborted because the caller's context
// was canceled before any event was persisted or in-memory state published.
type CanceledError struct {
	Cause error
}

func (e *CanceledError) Error() string {
	if e.Cause != nil {
		return "create session canceled before commit: " + e.Cause.Error()
	}
	return "create session canceled before commit"
}

func (e *CanceledError) Unwrap() error { return e.Cause }

func validateCommand(expectedVersion uint64, idempotencyKey string) error {
	if idempotencyKey == "" {
		return &RequestError{Code: "IDEMPOTENCY_KEY_REQUIRED", Message: "idempotencyKey 不能为空"}
	}
	return nil
}

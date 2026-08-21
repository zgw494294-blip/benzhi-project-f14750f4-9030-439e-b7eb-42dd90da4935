package application

import "fmt"

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

func validateCommand(expectedVersion uint64, idempotencyKey string) error {
	if idempotencyKey == "" {
		return &RequestError{Code: "IDEMPOTENCY_KEY_REQUIRED", Message: "idempotencyKey 不能为空"}
	}
	return nil
}

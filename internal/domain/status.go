package domain

import "fmt"

type SessionStatus string

const (
	SessionDraft      SessionStatus = "Draft"
	SessionPrepared   SessionStatus = "Prepared"
	SessionRunning    SessionStatus = "Running"
	SessionReview     SessionStatus = "Review"
	SessionCorrection SessionStatus = "Correction"
	SessionCertified  SessionStatus = "Certified"
)

type CueStatus string

const (
	CuePending CueStatus = "Pending"
	CuePassed  CueStatus = "Passed"
	CueFailed  CueStatus = "Failed"
)

type AttemptResult string

const (
	AttemptPass AttemptResult = "Pass"
	AttemptFail AttemptResult = "Fail"
)

type ReviewDecision string

const (
	ReviewApproved        ReviewDecision = "Approved"
	ReviewNeedsCorrection ReviewDecision = "NeedsCorrection"
)

type ViolationCode string

const (
	ViolationLoad      ViolationCode = "LOAD_EXCEEDS_RATED"
	ViolationClearance ViolationCode = "CLEARANCE_BELOW_MINIMUM"
	ViolationStopTime  ViolationCode = "EMERGENCY_STOP_TOO_SLOW"
)

type Violation struct {
	Code    ViolationCode `json:"code"`
	Field   string        `json:"field"`
	Actual  float64       `json:"actual"`
	Limit   float64       `json:"limit"`
	Message string        `json:"message"`
}

type RuleError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type ValidationIssue struct {
	Row     int    `json:"row,omitempty"`
	Entity  string `json:"entity,omitempty"`
	ID      string `json:"id,omitempty"`
	Field   string `json:"field"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

type ValidationError struct {
	Code     string            `json:"code"`
	Message  string            `json:"message"`
	Problems []ValidationIssue `json:"problems"`
}

func (e *ValidationError) Error() string { return e.Message }

func (e *RuleError) Error() string { return e.Message }

func ruleError(code, format string, args ...any) error {
	return &RuleError{Code: code, Message: fmt.Sprintf(format, args...)}
}

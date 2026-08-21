package domain

import (
	"fmt"
	"strings"
	"time"
)

type RecordAttemptInput struct {
	ID                  string
	CueID               string
	MeasuredLoadKg      float64
	MeasuredClearanceCm float64
	MeasuredStopMs      int
	Operator            string
	EvidenceNote        string
}

func EvaluateAttempt(device RiggingDevice, cue SafetyCue, input RecordAttemptInput) (AttemptResult, []Violation) {
	violations := make([]Violation, 0, 3)
	if input.MeasuredLoadKg > device.RatedLoadKg {
		violations = append(violations, Violation{Code: ViolationLoad, Field: "measuredLoadKg", Actual: input.MeasuredLoadKg, Limit: device.RatedLoadKg, Message: fmt.Sprintf("实测载荷 %.1f kg 超过额定载荷 %.1f kg", input.MeasuredLoadKg, device.RatedLoadKg)})
	}
	if input.MeasuredClearanceCm < cue.MinimumClearanceCm {
		violations = append(violations, Violation{Code: ViolationClearance, Field: "measuredClearanceCm", Actual: input.MeasuredClearanceCm, Limit: cue.MinimumClearanceCm, Message: fmt.Sprintf("实测净空 %.1f cm 低于最低净空 %.1f cm", input.MeasuredClearanceCm, cue.MinimumClearanceCm)})
	}
	if device.EmergencyStopRequired && input.MeasuredStopMs > cue.MaximumStopMs {
		violations = append(violations, Violation{Code: ViolationStopTime, Field: "measuredStopMs", Actual: float64(input.MeasuredStopMs), Limit: float64(cue.MaximumStopMs), Message: fmt.Sprintf("急停时间 %d ms 超过上限 %d ms", input.MeasuredStopMs, cue.MaximumStopMs)})
	}
	if len(violations) > 0 {
		return AttemptFail, violations
	}
	return AttemptPass, violations
}

func (a *Aggregate) RecordAttempt(input RecordAttemptInput, now time.Time) ([]Event, error) {
	if err := a.ensureStatus(SessionRunning); err != nil {
		return nil, err
	}
	input.ID = strings.TrimSpace(input.ID)
	input.CueID = strings.TrimSpace(input.CueID)
	input.Operator = strings.TrimSpace(input.Operator)
	input.EvidenceNote = strings.TrimSpace(input.EvidenceNote)
	if input.ID == "" || input.Operator == "" || input.EvidenceNote == "" {
		return nil, ruleError("INVALID_ATTEMPT", "实测记录 ID、操作员和现场证据不能为空")
	}
	for _, attempts := range a.Attempts {
		for _, attempt := range attempts {
			if attempt.ID == input.ID {
				return nil, ruleError("DUPLICATE_ATTEMPT", "实测记录 ID %s 已存在", input.ID)
			}
		}
	}
	if input.MeasuredLoadKg < 0 || input.MeasuredClearanceCm < 0 || input.MeasuredStopMs < 0 {
		return nil, ruleError("INVALID_MEASUREMENT", "实测数据不能为负数")
	}
	cue, exists := a.Cues[input.CueID]
	if !exists {
		return nil, ruleError("CUE_NOT_FOUND", "动作不存在")
	}
	next := a.NextPendingCue()
	if next == nil {
		return nil, ruleError("NO_PENDING_CUE", "当前没有待执行动作")
	}
	if next.ID != cue.ID {
		return nil, ruleError("CUE_OUT_OF_ORDER", "必须先执行序号 %d 的动作", next.Sequence)
	}
	device := a.Devices[cue.DeviceID]
	result, violations := EvaluateAttempt(device, cue, input)
	attempt := CueAttempt{ID: input.ID, CueID: cue.ID, AttemptNo: cue.AttemptCount + 1, MeasuredLoadKg: input.MeasuredLoadKg, MeasuredClearanceCm: input.MeasuredClearanceCm, MeasuredStopMs: input.MeasuredStopMs, Operator: input.Operator, EvidenceNote: input.EvidenceNote, Result: result, Violations: violations, RecordedAt: now.UTC()}
	recorded, err := a.emit(EventAttemptRecorded, now, AttemptRecorded{Attempt: attempt})
	if err != nil {
		return nil, err
	}
	events := []Event{recorded}
	if a.NextPendingCue() == nil {
		requested, err := a.emit(EventReviewRequested, now, ReviewRequested{})
		if err != nil {
			return nil, err
		}
		events = append(events, requested)
	}
	return events, nil
}

func (a *Aggregate) RecordAttemptBatch(inputs []RecordAttemptInput, now time.Time) ([]Event, error) {
	if err := a.ensureStatus(SessionRunning); err != nil {
		return nil, err
	}
	if len(inputs) == 0 {
		return nil, ruleError("ATTEMPT_BATCH_REQUIRED", "至少需要一条实测记录")
	}
	seenCues := make(map[string]bool, len(inputs))
	seenAttempts := make(map[string]bool, len(inputs))
	existingAttempts := make(map[string]bool)
	for _, attempts := range a.Attempts {
		for _, attempt := range attempts {
			existingAttempts[attempt.ID] = true
		}
	}
	problems := make([]ValidationIssue, 0)
	pending := make([]SafetyCue, 0)
	for _, cue := range a.OrderedCues() {
		if cue.Status == CuePending {
			pending = append(pending, cue)
		}
	}
	for index, input := range inputs {
		row := index + 1
		cueID := strings.TrimSpace(input.CueID)
		attemptID := strings.TrimSpace(input.ID)
		if seenCues[cueID] {
			problems = append(problems, ValidationIssue{Row: row, Entity: "attempt", ID: attemptID, Field: "cueID", Code: "DUPLICATE_CUE", Message: "同一批次不能重复提交 cueID"})
		}
		seenCues[cueID] = true
		if attemptID == "" || seenAttempts[attemptID] || existingAttempts[attemptID] {
			problems = append(problems, ValidationIssue{Row: row, Entity: "attempt", ID: attemptID, Field: "id", Code: "INVALID_ATTEMPT_ID", Message: "实测记录 ID 不能为空且批次内不能重复"})
		}
		seenAttempts[attemptID] = true
		if index >= len(pending) || pending[index].ID != cueID {
			expected := ""
			if index < len(pending) {
				expected = pending[index].ID
			}
			problems = append(problems, ValidationIssue{Row: row, Entity: "attempt", ID: attemptID, Field: "cueID", Code: "CUE_OUT_OF_ORDER", Message: fmt.Sprintf("必须从当前待办开始连续提交，期望 cueID %s", expected)})
		}
		if strings.TrimSpace(input.Operator) == "" {
			problems = append(problems, ValidationIssue{Row: row, Entity: "attempt", ID: attemptID, Field: "operator", Code: "REQUIRED", Message: "操作员不能为空"})
		}
		if strings.TrimSpace(input.EvidenceNote) == "" {
			problems = append(problems, ValidationIssue{Row: row, Entity: "attempt", ID: attemptID, Field: "evidenceNote", Code: "REQUIRED", Message: "现场证据不能为空"})
		}
		if input.MeasuredLoadKg < 0 || input.MeasuredClearanceCm < 0 || input.MeasuredStopMs < 0 {
			problems = append(problems, ValidationIssue{Row: row, Entity: "attempt", ID: attemptID, Field: "measurements", Code: "INVALID_MEASUREMENT", Message: "实测数据不能为负数"})
		}
	}
	if len(problems) > 0 {
		return nil, &ValidationError{Code: "ATTEMPT_BATCH_INVALID", Message: "连续实测批次校验未通过", Problems: problems}
	}
	shadow := a.Clone()
	events := make([]Event, 0, len(inputs)+1)
	for _, input := range inputs {
		generated, err := shadow.RecordAttempt(input, now)
		if err != nil {
			return nil, err
		}
		events = append(events, generated...)
	}
	*a = *shadow
	return events, nil
}

func (a *Aggregate) NextPendingCue() *SafetyCue {
	for _, cue := range a.OrderedCues() {
		if cue.Status == CuePending {
			current := cue
			return &current
		}
	}
	return nil
}

func (a *Aggregate) FailedCueIDs() []string {
	failed := make([]string, 0)
	for _, cue := range a.OrderedCues() {
		if cue.Status == CueFailed {
			failed = append(failed, cue.ID)
		}
	}
	return failed
}

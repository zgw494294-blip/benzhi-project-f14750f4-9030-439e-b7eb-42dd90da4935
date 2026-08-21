package domain

import (
	"slices"
	"strings"
	"time"
)

type CompleteReviewInput struct {
	ID             string
	Reviewer       string
	Decision       ReviewDecision
	Findings       []string
	CorrectionNote string
}

type UpdateCorrectionTaskInput struct {
	CueID        string
	Measure      string
	Owner        string
	EvidenceNote string
	Closed       bool
}

func (a *Aggregate) CompleteReview(input CompleteReviewInput, now time.Time) (Event, error) {
	if err := a.ensureStatus(SessionReview); err != nil {
		return Event{}, err
	}
	input.ID = strings.TrimSpace(input.ID)
	input.Reviewer = strings.TrimSpace(input.Reviewer)
	input.CorrectionNote = strings.TrimSpace(input.CorrectionNote)
	if input.ID == "" || input.Reviewer == "" {
		return Event{}, ruleError("INVALID_REVIEW", "复核 ID 和检查员不能为空")
	}
	cleanFindings := make([]string, 0, len(input.Findings))
	for _, finding := range input.Findings {
		if value := strings.TrimSpace(finding); value != "" {
			cleanFindings = append(cleanFindings, value)
		}
	}
	failed := a.FailedCueIDs()
	switch input.Decision {
	case ReviewApproved:
		if len(failed) != 0 {
			return Event{}, ruleError("FAILED_CUES_REMAIN", "仍有 %d 个动作未通过，不能批准", len(failed))
		}
	case ReviewNeedsCorrection:
		if len(failed) == 0 {
			return Event{}, ruleError("NO_FAILED_CUES", "没有失败动作可进入整改")
		}
		if len(cleanFindings) == 0 || input.CorrectionNote == "" {
			return Event{}, ruleError("CORRECTION_REASON_REQUIRED", "退回整改必须填写发现项和整改要求")
		}
	default:
		return Event{}, ruleError("INVALID_REVIEW_DECISION", "复核结论无效")
	}
	review := SafetyReview{ID: input.ID, SessionID: a.Session.ID, Reviewer: input.Reviewer, Decision: input.Decision, Findings: cleanFindings, CorrectionNote: input.CorrectionNote, ReviewedAt: now.UTC()}
	tasks := make([]CorrectionTask, 0, len(failed))
	if input.Decision == ReviewNeedsCorrection {
		for _, cueID := range failed {
			attempts := a.Attempts[cueID]
			if len(attempts) == 0 {
				return Event{}, ruleError("FAILED_ATTEMPT_REQUIRED", "失败动作 %s 缺少实测记录", cueID)
			}
			attempt := attempts[len(attempts)-1]
			tasks = append(tasks, CorrectionTask{CueID: cueID, AttemptID: attempt.ID, Violations: slices.Clone(attempt.Violations), UpdatedAt: now.UTC()})
		}
	}
	return a.emit(EventReviewCompleted, now, ReviewCompleted{Review: review, CorrectionTasks: tasks})
}

func (a *Aggregate) UpdateCorrectionTask(input UpdateCorrectionTaskInput, now time.Time) (Event, error) {
	if err := a.ensureStatus(SessionCorrection); err != nil {
		return Event{}, err
	}
	input.CueID = strings.TrimSpace(input.CueID)
	if !a.CorrectionCueIDs[input.CueID] {
		return Event{}, ruleError("CORRECTION_TASK_NOT_ALLOWED", "动作 %s 不属于本轮整改", input.CueID)
	}
	before, exists := a.CorrectionTasks[input.CueID]
	if !exists {
		return Event{}, ruleError("CORRECTION_TASK_NOT_FOUND", "动作 %s 的整改任务不存在", input.CueID)
	}
	after := before
	after.Measure = strings.TrimSpace(input.Measure)
	after.Owner = strings.TrimSpace(input.Owner)
	after.EvidenceNote = strings.TrimSpace(input.EvidenceNote)
	after.UpdatedAt = now.UTC()
	if input.Closed {
		closed := now.UTC()
		after.ClosedAt = &closed
	} else {
		after.ClosedAt = nil
	}
	return a.emit(EventCorrectionTaskUpdated, now, CorrectionTaskUpdated{Before: before, After: after})
}

func (a *Aggregate) SubmitCorrection(note string, now time.Time) ([]Event, error) {
	if err := a.ensureStatus(SessionCorrection); err != nil {
		return nil, err
	}
	note = strings.TrimSpace(note)
	if len(a.CorrectionCueIDs) == 0 {
		return nil, ruleError("NO_CORRECTION_CUES", "没有可重测的失败动作")
	}
	ids := make([]string, 0, len(a.CorrectionCueIDs))
	problems := make([]ValidationIssue, 0)
	for cueID := range a.CorrectionCueIDs {
		ids = append(ids, cueID)
		task, exists := a.CorrectionTasks[cueID]
		if !exists {
			problems = append(problems, ValidationIssue{Entity: "correctionTask", ID: cueID, Field: "task", Code: "TASK_MISSING", Message: "整改任务不存在"})
			continue
		}
		if task.Owner == "" {
			problems = append(problems, ValidationIssue{Entity: "correctionTask", ID: cueID, Field: "owner", Code: "REQUIRED", Message: "负责人不能为空"})
		}
		if task.Measure == "" {
			problems = append(problems, ValidationIssue{Entity: "correctionTask", ID: cueID, Field: "measure", Code: "REQUIRED", Message: "整改措施不能为空"})
		}
		if task.EvidenceNote == "" {
			problems = append(problems, ValidationIssue{Entity: "correctionTask", ID: cueID, Field: "evidenceNote", Code: "REQUIRED", Message: "完成证据不能为空"})
		}
		if task.ClosedAt == nil {
			problems = append(problems, ValidationIssue{Entity: "correctionTask", ID: cueID, Field: "closedAt", Code: "REQUIRED", Message: "整改任务尚未关闭"})
		}
	}
	if len(problems) > 0 {
		return nil, &ValidationError{Code: "CORRECTION_TASKS_INCOMPLETE", Message: "仍有未完成的整改任务", Problems: problems}
	}
	slices.Sort(ids)
	submitted, err := a.emit(EventCorrectionSubmitted, now, CorrectionSubmitted{Note: note, TaskCueIDs: ids})
	if err != nil {
		return nil, err
	}
	started, err := a.emit(EventRunStarted, now, RunStarted{Correction: true})
	if err != nil {
		return nil, err
	}
	return []Event{submitted, started}, nil
}

func (a *Aggregate) LatestApprovedReviewer() (string, bool) {
	for index := len(a.Reviews) - 1; index >= 0; index-- {
		if a.Reviews[index].Decision == ReviewApproved {
			return a.Reviews[index].Reviewer, true
		}
	}
	return "", false
}

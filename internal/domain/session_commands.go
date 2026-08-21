package domain

import (
	"strings"
	"time"
)

type CreateSessionInput struct {
	ID                string
	ProductionName    string
	Venue             string
	PerformanceDate   time.Time
	TechnicalDirector string
}

func CreateSession(input CreateSessionInput, now time.Time) (*Aggregate, Event, error) {
	if strings.TrimSpace(input.ID) == "" {
		return nil, Event{}, ruleError("INVALID_ID", "会话 ID 不能为空")
	}
	if strings.TrimSpace(input.ProductionName) == "" {
		return nil, Event{}, ruleError("INVALID_PRODUCTION_NAME", "演出名称不能为空")
	}
	if strings.TrimSpace(input.Venue) == "" {
		return nil, Event{}, ruleError("INVALID_VENUE", "场地不能为空")
	}
	if input.PerformanceDate.IsZero() {
		return nil, Event{}, ruleError("INVALID_PERFORMANCE_DATE", "演出日期不能为空")
	}
	if strings.TrimSpace(input.TechnicalDirector) == "" {
		return nil, Event{}, ruleError("INVALID_TECHNICAL_DIRECTOR", "技术负责人不能为空")
	}
	aggregate := NewAggregate()
	session := ValidationSession{
		ID: input.ID, ProductionName: strings.TrimSpace(input.ProductionName),
		Venue: strings.TrimSpace(input.Venue), PerformanceDate: input.PerformanceDate.UTC(),
		TechnicalDirector: strings.TrimSpace(input.TechnicalDirector),
		Status:            SessionDraft, CreatedAt: now.UTC(),
	}
	event, err := MakeEvent(EventSessionCreated, input.ID, 1, now, SessionCreated{Session: session})
	if err != nil {
		return nil, Event{}, err
	}
	if err := aggregate.Apply(event); err != nil {
		return nil, Event{}, err
	}
	return aggregate, event, nil
}

func (a *Aggregate) ensureStatus(allowed ...SessionStatus) error {
	for _, status := range allowed {
		if a.Session.Status == status {
			return nil
		}
	}
	return ruleError("INVALID_STATUS", "当前状态 %s 不允许此操作", a.Session.Status)
}

func (a *Aggregate) Prepare(now time.Time) (Event, error) {
	if err := a.ensureStatus(SessionDraft); err != nil {
		return Event{}, err
	}
	if len(a.Devices) == 0 {
		return Event{}, ruleError("DEVICE_REQUIRED", "至少需要配置一台吊挂设备")
	}
	if len(a.Cues) == 0 {
		return Event{}, ruleError("CUE_REQUIRED", "至少需要配置一个安全动作")
	}
	for index, cue := range a.OrderedCues() {
		if cue.Sequence != index+1 {
			return Event{}, ruleError("CUE_SEQUENCE_GAP", "动作序号必须从 1 开始连续排列")
		}
	}
	return a.emit(EventConfigurationReady, now, ConfigurationPrepared{})
}

func (a *Aggregate) StartRun(now time.Time) (Event, error) {
	if err := a.ensureStatus(SessionPrepared); err != nil {
		return Event{}, err
	}
	return a.emit(EventRunStarted, now, RunStarted{})
}

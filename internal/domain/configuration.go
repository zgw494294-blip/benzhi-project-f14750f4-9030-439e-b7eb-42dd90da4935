package domain

import (
	"fmt"
	"slices"
	"strings"
	"time"
)

type BatchConfigurationInput struct {
	Devices []RiggingDevice `json:"devices"`
	Cues    []SafetyCue     `json:"cues"`
}

type ConfigurationPreflight struct {
	Valid       bool              `json:"valid"`
	DeviceCount int               `json:"deviceCount"`
	CueCount    int               `json:"cueCount"`
	Problems    []ValidationIssue `json:"problems"`
}

func (a *Aggregate) AddDevice(device RiggingDevice, now time.Time) (Event, error) {
	if err := a.ensureStatus(SessionDraft); err != nil {
		return Event{}, err
	}
	device.ID = strings.TrimSpace(device.ID)
	device.Name = strings.TrimSpace(device.Name)
	device.DeviceType = strings.TrimSpace(device.DeviceType)
	device.SafeZone = strings.TrimSpace(device.SafeZone)
	if device.ID == "" {
		return Event{}, ruleError("INVALID_DEVICE_ID", "设备 ID 不能为空")
	}
	if _, exists := a.Devices[device.ID]; exists {
		return Event{}, ruleError("DUPLICATE_DEVICE", "设备 %s 已存在", device.ID)
	}
	if device.Name == "" || device.DeviceType == "" || device.SafeZone == "" {
		return Event{}, ruleError("INVALID_DEVICE", "设备名称、类型和安全区域不能为空")
	}
	if device.RatedLoadKg <= 0 {
		return Event{}, ruleError("INVALID_RATED_LOAD", "设备额定载荷必须大于 0")
	}
	device.SessionID = a.Session.ID
	return a.emit(EventDeviceAdded, now, DeviceAdded{Device: device})
}

func (a *Aggregate) UpdateDevice(device RiggingDevice, now time.Time) (Event, error) {
	if err := a.ensureStatus(SessionDraft); err != nil {
		return Event{}, err
	}
	device.ID = strings.TrimSpace(device.ID)
	before, exists := a.Devices[device.ID]
	if !exists {
		return Event{}, ruleError("DEVICE_NOT_FOUND", "设备 %s 不存在", device.ID)
	}
	device.Name = strings.TrimSpace(device.Name)
	device.DeviceType = strings.TrimSpace(device.DeviceType)
	device.SafeZone = strings.TrimSpace(device.SafeZone)
	if device.Name == "" || device.DeviceType == "" || device.SafeZone == "" {
		return Event{}, ruleError("INVALID_DEVICE", "设备名称、类型和安全区域不能为空")
	}
	if device.RatedLoadKg <= 0 {
		return Event{}, ruleError("INVALID_RATED_LOAD", "设备额定载荷必须大于 0")
	}
	device.SessionID = a.Session.ID
	problems := make([]ValidationIssue, 0)
	for _, cue := range a.OrderedCues() {
		if cue.DeviceID != device.ID {
			continue
		}
		if cue.ExpectedLoadKg > device.RatedLoadKg {
			problems = append(problems, ValidationIssue{Entity: "cue", ID: cue.ID, Field: "expectedLoadKg", Code: "LOAD_EXCEEDS_RATED", Message: fmt.Sprintf("动作 %s 的预期载荷 %.1f kg 超过新额定载荷 %.1f kg", cue.ID, cue.ExpectedLoadKg, device.RatedLoadKg)})
		}
		if device.EmergencyStopRequired && cue.MaximumStopMs <= 0 {
			problems = append(problems, ValidationIssue{Entity: "cue", ID: cue.ID, Field: "maximumStopMs", Code: "STOP_LIMIT_REQUIRED", Message: fmt.Sprintf("动作 %s 缺少新设备要求的最大急停时间", cue.ID)})
		}
	}
	if len(problems) > 0 {
		return Event{}, &ValidationError{Code: "DEVICE_UPDATE_CONFLICT", Message: "设备修订与关联动作冲突", Problems: problems}
	}
	return a.emit(EventDeviceUpdated, now, DeviceUpdated{Before: before, After: device})
}

func (a *Aggregate) DeleteDevice(id string, now time.Time) (Event, error) {
	if err := a.ensureStatus(SessionDraft); err != nil {
		return Event{}, err
	}
	id = strings.TrimSpace(id)
	device, exists := a.Devices[id]
	if !exists {
		return Event{}, ruleError("DEVICE_NOT_FOUND", "设备 %s 不存在", id)
	}
	for _, cue := range a.Cues {
		if cue.DeviceID == id {
			return Event{}, &ValidationError{Code: "DEVICE_IN_USE", Message: "设备仍被动作引用，不能删除", Problems: []ValidationIssue{{Entity: "cue", ID: cue.ID, Field: "deviceID", Code: "DEVICE_IN_USE", Message: fmt.Sprintf("动作 %s 正在引用设备 %s", cue.ID, id)}}}
		}
	}
	return a.emit(EventDeviceDeleted, now, DeviceDeleted{Device: device})
}

func (a *Aggregate) AddCue(cue SafetyCue, now time.Time) (Event, error) {
	if err := a.ensureStatus(SessionDraft); err != nil {
		return Event{}, err
	}
	cue.ID = strings.TrimSpace(cue.ID)
	cue.DeviceID = strings.TrimSpace(cue.DeviceID)
	cue.Action = strings.TrimSpace(cue.Action)
	if cue.ID == "" {
		return Event{}, ruleError("INVALID_CUE_ID", "动作 ID 不能为空")
	}
	if _, exists := a.Cues[cue.ID]; exists {
		return Event{}, ruleError("DUPLICATE_CUE", "动作 %s 已存在", cue.ID)
	}
	device, exists := a.Devices[cue.DeviceID]
	if !exists {
		return Event{}, ruleError("DEVICE_NOT_FOUND", "动作引用的设备不存在")
	}
	if cue.Sequence != len(a.Cues)+1 {
		return Event{}, ruleError("INVALID_SEQUENCE", "下一个动作序号必须为 %d", len(a.Cues)+1)
	}
	if cue.Action == "" {
		return Event{}, ruleError("INVALID_ACTION", "动作说明不能为空")
	}
	if cue.ExpectedLoadKg <= 0 || cue.ExpectedLoadKg > device.RatedLoadKg {
		return Event{}, ruleError("INVALID_EXPECTED_LOAD", "预期载荷必须大于 0 且不超过设备额定载荷 %.1f kg", device.RatedLoadKg)
	}
	if cue.MinimumClearanceCm <= 0 {
		return Event{}, ruleError("INVALID_CLEARANCE", "最小净空必须大于 0")
	}
	if device.EmergencyStopRequired && cue.MaximumStopMs <= 0 {
		return Event{}, ruleError("INVALID_STOP_LIMIT", "需要急停的设备必须设置最大急停时间")
	}
	if !device.EmergencyStopRequired && cue.MaximumStopMs < 0 {
		return Event{}, ruleError("INVALID_STOP_LIMIT", "最大急停时间不能为负数")
	}
	cue.SessionID = a.Session.ID
	cue.Status = CuePending
	return a.emit(EventCueAdded, now, CueAdded{Cue: cue})
}

func (a *Aggregate) UpdateCue(cue SafetyCue, now time.Time) ([]Event, error) {
	if err := a.ensureStatus(SessionDraft); err != nil {
		return nil, err
	}
	cue.ID = strings.TrimSpace(cue.ID)
	before, exists := a.Cues[cue.ID]
	if !exists {
		return nil, ruleError("CUE_NOT_FOUND", "动作 %s 不存在", cue.ID)
	}
	targetSequence := cue.Sequence
	if targetSequence < 1 || targetSequence > len(a.Cues) {
		return nil, ruleError("INVALID_SEQUENCE", "动作序号必须在 1 到 %d 之间", len(a.Cues))
	}
	device, exists := a.Devices[strings.TrimSpace(cue.DeviceID)]
	if !exists {
		return nil, ruleError("DEVICE_NOT_FOUND", "动作引用的设备不存在")
	}
	cue.DeviceID = device.ID
	cue.Action = strings.TrimSpace(cue.Action)
	if cue.Action == "" {
		return nil, ruleError("INVALID_ACTION", "动作说明不能为空")
	}
	if cue.ExpectedLoadKg <= 0 || cue.ExpectedLoadKg > device.RatedLoadKg {
		return nil, ruleError("INVALID_EXPECTED_LOAD", "预期载荷必须大于 0 且不超过设备额定载荷 %.1f kg", device.RatedLoadKg)
	}
	if cue.MinimumClearanceCm <= 0 {
		return nil, ruleError("INVALID_CLEARANCE", "最小净空必须大于 0")
	}
	if device.EmergencyStopRequired && cue.MaximumStopMs <= 0 || !device.EmergencyStopRequired && cue.MaximumStopMs < 0 {
		return nil, ruleError("INVALID_STOP_LIMIT", "最大急停时间与设备急停要求不一致")
	}
	cue.SessionID = a.Session.ID
	cue.Status = before.Status
	cue.AttemptCount = before.AttemptCount
	cue.Sequence = before.Sequence
	updated, err := a.emit(EventCueUpdated, now, CueUpdated{Before: before, After: cue})
	if err != nil {
		return nil, err
	}
	events := []Event{updated}
	if targetSequence != before.Sequence {
		beforeOrder := cueIDs(a.OrderedCues())
		afterOrder := slices.Clone(beforeOrder)
		current := slices.Index(afterOrder, cue.ID)
		afterOrder = slices.Delete(afterOrder, current, current+1)
		target := targetSequence - 1
		afterOrder = slices.Insert(afterOrder, target, cue.ID)
		reordered, err := a.emit(EventCuesReordered, now, CuesReordered{Before: beforeOrder, After: afterOrder})
		if err != nil {
			return nil, err
		}
		events = append(events, reordered)
	}
	return events, nil
}

func (a *Aggregate) DeleteCue(id string, now time.Time) (Event, error) {
	if err := a.ensureStatus(SessionDraft); err != nil {
		return Event{}, err
	}
	id = strings.TrimSpace(id)
	cue, exists := a.Cues[id]
	if !exists {
		return Event{}, ruleError("CUE_NOT_FOUND", "动作 %s 不存在", id)
	}
	return a.emit(EventCueDeleted, now, CueDeleted{Cue: cue})
}

func (a *Aggregate) ReorderCues(ids []string, now time.Time) (Event, error) {
	if err := a.ensureStatus(SessionDraft); err != nil {
		return Event{}, err
	}
	if len(ids) != len(a.Cues) {
		return Event{}, ruleError("INVALID_CUE_ORDER", "动作顺序必须包含全部 %d 个动作", len(a.Cues))
	}
	after := make([]string, len(ids))
	seen := make(map[string]bool, len(ids))
	for index, raw := range ids {
		id := strings.TrimSpace(raw)
		if _, exists := a.Cues[id]; !exists || seen[id] {
			return Event{}, ruleError("INVALID_CUE_ORDER", "动作顺序包含不存在或重复的 cueID %s", id)
		}
		seen[id] = true
		after[index] = id
	}
	before := cueIDs(a.OrderedCues())
	return a.emit(EventCuesReordered, now, CuesReordered{Before: before, After: after})
}

func cueIDs(cues []SafetyCue) []string {
	ids := make([]string, len(cues))
	for index, cue := range cues {
		ids[index] = cue.ID
	}
	return ids
}

func (a *Aggregate) PreflightConfiguration(input BatchConfigurationInput) ConfigurationPreflight {
	report := ConfigurationPreflight{DeviceCount: len(input.Devices), CueCount: len(input.Cues)}
	if len(input.Devices) == 0 && len(input.Cues) == 0 {
		report.Problems = append(report.Problems, ValidationIssue{Field: "batch", Code: "BATCH_EMPTY", Message: "批量配置至少需要一台设备或一个动作"})
	}
	if a.Session.Status != SessionDraft {
		report.Problems = append(report.Problems, ValidationIssue{Field: "status", Code: "INVALID_STATUS", Message: fmt.Sprintf("当前状态 %s 不允许批量配置", a.Session.Status)})
	}
	devices := make(map[string]RiggingDevice, len(a.Devices)+len(input.Devices))
	for id, device := range a.Devices {
		devices[id] = device
	}
	batchDeviceIDs := make(map[string]bool)
	for index, raw := range input.Devices {
		row := index + 1
		device := raw
		device.ID = strings.TrimSpace(device.ID)
		if device.ID == "" {
			report.Problems = append(report.Problems, ValidationIssue{Row: row, Entity: "device", Field: "id", Code: "INVALID_DEVICE_ID", Message: "设备 ID 不能为空"})
		} else if _, exists := devices[device.ID]; exists || batchDeviceIDs[device.ID] {
			report.Problems = append(report.Problems, ValidationIssue{Row: row, Entity: "device", ID: device.ID, Field: "id", Code: "DUPLICATE_DEVICE", Message: "设备 ID 已存在或在批次中重复"})
		} else {
			batchDeviceIDs[device.ID] = true
			devices[device.ID] = device
		}
		if strings.TrimSpace(device.Name) == "" {
			report.Problems = append(report.Problems, ValidationIssue{Row: row, Entity: "device", ID: device.ID, Field: "name", Code: "REQUIRED", Message: "设备名称不能为空"})
		}
		if strings.TrimSpace(device.DeviceType) == "" {
			report.Problems = append(report.Problems, ValidationIssue{Row: row, Entity: "device", ID: device.ID, Field: "deviceType", Code: "REQUIRED", Message: "设备类型不能为空"})
		}
		if strings.TrimSpace(device.SafeZone) == "" {
			report.Problems = append(report.Problems, ValidationIssue{Row: row, Entity: "device", ID: device.ID, Field: "safeZone", Code: "REQUIRED", Message: "安全区域不能为空"})
		}
		if device.RatedLoadKg <= 0 {
			report.Problems = append(report.Problems, ValidationIssue{Row: row, Entity: "device", ID: device.ID, Field: "ratedLoadKg", Code: "INVALID_RATED_LOAD", Message: "额定载荷必须大于 0"})
		}
	}
	batchCueIDs := make(map[string]bool)
	sequences := make(map[int]int)
	for _, cue := range a.Cues {
		sequences[cue.Sequence]++
	}
	for index, raw := range input.Cues {
		row := index + 1
		cue := raw
		cue.ID = strings.TrimSpace(cue.ID)
		cue.DeviceID = strings.TrimSpace(cue.DeviceID)
		if cue.ID == "" {
			report.Problems = append(report.Problems, ValidationIssue{Row: row, Entity: "cue", Field: "id", Code: "INVALID_CUE_ID", Message: "动作 ID 不能为空"})
		} else if _, exists := a.Cues[cue.ID]; exists || batchCueIDs[cue.ID] {
			report.Problems = append(report.Problems, ValidationIssue{Row: row, Entity: "cue", ID: cue.ID, Field: "id", Code: "DUPLICATE_CUE", Message: "动作 ID 已存在或在批次中重复"})
		} else {
			batchCueIDs[cue.ID] = true
		}
		sequences[cue.Sequence]++
		device, deviceExists := devices[cue.DeviceID]
		if !deviceExists {
			report.Problems = append(report.Problems, ValidationIssue{Row: row, Entity: "cue", ID: cue.ID, Field: "deviceID", Code: "DEVICE_NOT_FOUND", Message: "动作引用的设备不存在"})
		}
		if strings.TrimSpace(cue.Action) == "" {
			report.Problems = append(report.Problems, ValidationIssue{Row: row, Entity: "cue", ID: cue.ID, Field: "action", Code: "REQUIRED", Message: "动作说明不能为空"})
		}
		if cue.ExpectedLoadKg <= 0 || deviceExists && cue.ExpectedLoadKg > device.RatedLoadKg {
			report.Problems = append(report.Problems, ValidationIssue{Row: row, Entity: "cue", ID: cue.ID, Field: "expectedLoadKg", Code: "INVALID_EXPECTED_LOAD", Message: "预期载荷必须大于 0 且不超过设备额定载荷"})
		}
		if cue.MinimumClearanceCm <= 0 {
			report.Problems = append(report.Problems, ValidationIssue{Row: row, Entity: "cue", ID: cue.ID, Field: "minimumClearanceCm", Code: "INVALID_CLEARANCE", Message: "最小净空必须大于 0"})
		}
		if deviceExists && device.EmergencyStopRequired && cue.MaximumStopMs <= 0 || cue.MaximumStopMs < 0 {
			report.Problems = append(report.Problems, ValidationIssue{Row: row, Entity: "cue", ID: cue.ID, Field: "maximumStopMs", Code: "INVALID_STOP_LIMIT", Message: "最大急停时间与设备要求不一致"})
		}
	}
	totalCues := len(a.Cues) + len(input.Cues)
	for sequence := 1; sequence <= totalCues; sequence++ {
		if sequences[sequence] != 1 {
			report.Problems = append(report.Problems, ValidationIssue{Entity: "cue", Field: "sequence", Code: "INVALID_SEQUENCE", Message: fmt.Sprintf("动作序号 %d 必须且只能出现一次", sequence)})
		}
	}
	for sequence, count := range sequences {
		if sequence < 1 || sequence > totalCues || count > 1 {
			report.Problems = append(report.Problems, ValidationIssue{Entity: "cue", Field: "sequence", Code: "INVALID_SEQUENCE", Message: fmt.Sprintf("动作序号 %d 超出连续范围或重复", sequence)})
		}
	}
	report.Valid = len(report.Problems) == 0
	return report
}

func (a *Aggregate) ConfirmConfigurationBatch(input BatchConfigurationInput, now time.Time) ([]Event, ConfigurationPreflight, error) {
	report := a.PreflightConfiguration(input)
	if !report.Valid {
		return nil, report, &ValidationError{Code: "BATCH_VALIDATION_FAILED", Message: "批量配置预检未通过", Problems: report.Problems}
	}
	shadow := a.Clone()
	devices := slices.Clone(input.Devices)
	slices.SortFunc(devices, func(left, right RiggingDevice) int { return strings.Compare(left.ID, right.ID) })
	cues := slices.Clone(input.Cues)
	slices.SortFunc(cues, func(left, right SafetyCue) int {
		if left.Sequence != right.Sequence {
			return left.Sequence - right.Sequence
		}
		return strings.Compare(left.ID, right.ID)
	})
	events := make([]Event, 0, len(devices)+len(cues))
	for _, device := range devices {
		event, err := shadow.AddDevice(device, now)
		if err != nil {
			return nil, report, err
		}
		events = append(events, event)
	}
	for _, cue := range cues {
		event, err := shadow.AddCue(cue, now)
		if err != nil {
			return nil, report, err
		}
		events = append(events, event)
	}
	*a = *shadow
	return events, report, nil
}

func (a *Aggregate) OrderedCues() []SafetyCue {
	ordered := make([]SafetyCue, 0, len(a.Cues))
	for _, cue := range a.Cues {
		ordered = append(ordered, cue)
	}
	slices.SortFunc(ordered, func(left, right SafetyCue) int { return left.Sequence - right.Sequence })
	return ordered
}

func (a *Aggregate) OrderedDevices() []RiggingDevice {
	ordered := make([]RiggingDevice, 0, len(a.Session.DeviceIDs))
	for _, id := range a.Session.DeviceIDs {
		if device, exists := a.Devices[id]; exists {
			ordered = append(ordered, device)
		}
	}
	return ordered
}

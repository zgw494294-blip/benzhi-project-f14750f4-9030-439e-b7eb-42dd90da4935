package journal

import "fmt"

type ChainVerification struct {
	Valid                bool   `json:"valid"`
	CheckedThrough       uint64 `json:"checkedThrough"`
	FirstFailureSequence uint64 `json:"firstFailureSequence,omitempty"`
	Field                string `json:"field,omitempty"`
	Expected             string `json:"expected,omitempty"`
	Actual               string `json:"actual,omitempty"`
	Message              string `json:"message"`
}

func (s *Store) VerifyPrefix(throughSequence uint64) ChainVerification {
	s.mu.Lock()
	defer s.mu.Unlock()
	if throughSequence > uint64(len(s.records)) {
		return ChainVerification{Message: "验真范围超过日志末尾", FirstFailureSequence: uint64(len(s.records)) + 1, Field: "sequence", Expected: fmt.Sprint(throughSequence), Actual: fmt.Sprint(len(s.records))}
	}
	previous := ""
	versions := make(map[string]uint64)
	for index := uint64(0); index < throughSequence; index++ {
		record := s.records[index]
		sequence := index + 1
		if record.Sequence != sequence {
			return chainFailure(sequence, "sequence", fmt.Sprint(sequence), fmt.Sprint(record.Sequence), "日志 sequence 不连续")
		}
		if record.PreviousHash != previous {
			return chainFailure(sequence, "previousHash", previous, record.PreviousHash, "previousHash 未绑定上一条 checksum")
		}
		expectedChecksum, err := checksum(record)
		if err != nil {
			return chainFailure(sequence, "checksum", "可重新计算", err.Error(), "无法重新计算 checksum")
		}
		if record.Checksum != expectedChecksum {
			return chainFailure(sequence, "checksum", expectedChecksum, record.Checksum, "checksum 不匹配")
		}
		expectedVersion := versions[record.Event.SessionID] + 1
		if record.Event.Version != expectedVersion {
			return chainFailure(sequence, "sessionVersion", fmt.Sprint(expectedVersion), fmt.Sprint(record.Event.Version), "会话版本不连续")
		}
		versions[record.Event.SessionID] = record.Event.Version
		previous = record.Checksum
	}
	return ChainVerification{Valid: true, CheckedThrough: throughSequence, Message: "签发前事件链连续且校验和有效"}
}

func chainFailure(sequence uint64, field, expected, actual, message string) ChainVerification {
	return ChainVerification{FirstFailureSequence: sequence, Field: field, Expected: expected, Actual: actual, Message: message}
}

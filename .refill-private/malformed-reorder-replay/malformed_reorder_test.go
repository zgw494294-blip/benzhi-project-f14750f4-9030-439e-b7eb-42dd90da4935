package malformed_reorder_replay

import (
	"os"
	"strings"
	"testing"
	"time"

	"stageready/internal/application"
	"stageready/internal/domain"
	"stageready/internal/journal"
)

func TestMalformedReorderEventRejectedDuringReplay(t *testing.T) {
	dir := t.TempDir()
	store, err := journal.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Unix(1700000000, 0).UTC()
	aggregate, created, err := domain.CreateSession(domain.CreateSessionInput{
		ID: "session-reorder", ProductionName: "演出", Venue: "主舞台", PerformanceDate: now, TechnicalDirector: "负责人",
	}, now)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Append("session-reorder", 0, "create", []domain.Event{created}); err != nil {
		t.Fatal(err)
	}
	device, err := aggregate.AddDevice(domain.RiggingDevice{ID: "device-1", Name: "吊杆", DeviceType: "电动", RatedLoadKg: 500, SafeZone: "A"}, now)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Append("session-reorder", 1, "device", []domain.Event{device}); err != nil {
		t.Fatal(err)
	}
	cue1, err := aggregate.AddCue(domain.SafetyCue{ID: "cue-1", Sequence: 1, DeviceID: "device-1", Action: "上升", ExpectedLoadKg: 100, MinimumClearanceCm: 50}, now)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Append("session-reorder", 2, "cue-1", []domain.Event{cue1}); err != nil {
		t.Fatal(err)
	}
	cue2, err := aggregate.AddCue(domain.SafetyCue{ID: "cue-2", Sequence: 2, DeviceID: "device-1", Action: "下降", ExpectedLoadKg: 100, MinimumClearanceCm: 50}, now)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Append("session-reorder", 3, "cue-2", []domain.Event{cue2}); err != nil {
		t.Fatal(err)
	}
	malformed, err := domain.MakeEvent(domain.EventCuesReordered, "session-reorder", 5, now, domain.CuesReordered{
		Before: []string{"cue-1", "cue-2"}, After: []string{"cue-1"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Append("session-reorder", 4, "malformed-reorder", []domain.Event{malformed}); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(dir + "/snapshot.json"); err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}

	reopened, err := journal.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	_, err = application.NewService(reopened, func() time.Time { return now })
	if err == nil {
		t.Fatalf("malformed reorder event was accepted: replay produced a projection")
	}
	if !strings.Contains(err.Error(), "reorder event must contain all cues") {
		t.Fatalf("malformed reorder event returned unrelated error: %v", err)
	}
}

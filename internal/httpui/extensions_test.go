package httpui

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"stageready/internal/application"
	"stageready/internal/journal"
)

func TestExtendedRoutesReachBatchConfigurationAndRiskQueue(t *testing.T) {
	store, err := journal.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	service, err := application.NewService(store, func() time.Time { return time.Date(2026, 8, 21, 10, 0, 0, 0, time.UTC) })
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	server := httptest.NewServer(New(service).Handler())
	defer server.Close()

	request := func(method, path string, payload any, target any) int {
		t.Helper()
		var body bytes.Buffer
		if payload != nil {
			if err := json.NewEncoder(&body).Encode(payload); err != nil {
				t.Fatal(err)
			}
		}
		req, err := http.NewRequest(method, server.URL+path, &body)
		if err != nil {
			t.Fatal(err)
		}
		if payload != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		response, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer response.Body.Close()
		if target != nil {
			if err := json.NewDecoder(response.Body).Decode(target); err != nil {
				t.Fatal(err)
			}
		}
		return response.StatusCode
	}

	var created application.CommandResult
	status := request(http.MethodPost, "/api/sessions", map[string]any{"id": "http-session", "productionName": "HTTP 联调演出", "venue": "主舞台", "performanceDate": "2026-08-25T00:00:00Z", "technicalDirector": "总监", "expectedVersion": 0, "idempotencyKey": "http-create"}, &created)
	if status != http.StatusCreated {
		t.Fatalf("create status = %d", status)
	}
	batch := map[string]any{"devices": []map[string]any{{"id": "d1", "name": "吊杆", "deviceType": "电动", "ratedLoadKg": 500, "safeZone": "A", "emergencyStopRequired": true}}, "cues": []map[string]any{{"id": "c1", "sequence": 1, "deviceID": "d1", "action": "上升", "expectedLoadKg": 300, "minimumClearanceCm": 80, "maximumStopMs": 500}}}
	var preflight struct {
		Valid bool `json:"valid"`
	}
	if status := request(http.MethodPost, "/api/sessions/http-session/configuration/preflight", batch, &preflight); status != http.StatusOK || !preflight.Valid {
		t.Fatalf("preflight = %d / %#v", status, preflight)
	}
	batch["expectedVersion"] = created.Detail.Session.Version
	batch["idempotencyKey"] = "http-batch"
	var confirmed application.CommandResult
	if status := request(http.MethodPost, "/api/sessions/http-session/configuration/batch", batch, &confirmed); status != http.StatusOK || len(confirmed.Detail.Cues) != 1 {
		t.Fatalf("batch confirm = %d / %#v", status, confirmed)
	}
	var queue struct {
		Total   int `json:"total"`
		Summary struct {
			Pending int `json:"pendingCueCount"`
		} `json:"summary"`
	}
	if status := request(http.MethodGet, "/api/sessions?status=Draft&q=HTTP&page=1&pageSize=10", nil, &queue); status != http.StatusOK || queue.Total != 1 || queue.Summary.Pending != 1 {
		t.Fatalf("queue = %d / %#v", status, queue)
	}
	var invalid map[string]any
	if status := request(http.MethodGet, "/api/sessions?page=-1", nil, &invalid); status != http.StatusBadRequest {
		t.Fatalf("invalid query status = %d", status)
	}
}

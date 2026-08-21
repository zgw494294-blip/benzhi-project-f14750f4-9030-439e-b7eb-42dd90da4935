package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"stageready/internal/application"
	"stageready/internal/httpui"
	"stageready/internal/journal"
)

func main() {
	address := flag.String("addr", "127.0.0.1:19081", "HTTP 监听地址")
	selfcheck := flag.Bool("selfcheck", false, "执行完整业务自检并退出")
	dataDir := flag.String("data-dir", "", "事件日志目录")
	flag.Parse()
	resolvedAddress, err := resolveAddress(*address)
	if err != nil {
		slog.Error("地址配置无效", "error", err)
		os.Exit(2)
	}
	if *selfcheck {
		if err := runSelfcheck(resolvedAddress); err != nil {
			slog.Error("selfcheck 失败", "error", err)
			os.Exit(1)
		}
		return
	}
	if err := runServer(resolvedAddress, *dataDir); err != nil {
		slog.Error("服务退出", "error", err)
		os.Exit(1)
	}
}

func resolveAddress(argument string) (string, error) {
	address := strings.TrimSpace(argument)
	if address == "" {
		address = "127.0.0.1:19081"
	}
	if port := strings.TrimSpace(os.Getenv("PORT")); port != "" && address == "127.0.0.1:19081" {
		number, err := strconv.Atoi(port)
		if err != nil || number < 1 || number > 65535 {
			return "", errors.New("PORT 必须是 1-65535 的端口号")
		}
		address = "127.0.0.1:" + port
	}
	if strings.HasPrefix(address, ":") {
		return "", errors.New("监听地址必须显式绑定回环地址")
	}
	if !strings.HasPrefix(address, "127.0.0.1:") {
		return "", errors.New("服务只允许绑定 127.0.0.1")
	}
	return address, nil
}

func runServer(address, dataDir string) error {
	if dataDir == "" {
		dataDir = filepath.Join("data", "stageready")
	}
	store, err := journal.Open(dataDir)
	if err != nil {
		return err
	}
	service, err := application.NewService(store, time.Now)
	if err != nil {
		_ = store.Close()
		return err
	}
	defer service.Close()
	server := &http.Server{Addr: address, Handler: httpui.New(service).Handler(), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: 60 * time.Second}
	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(shutdown)
	go func() {
		<-shutdown
		context, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = server.Shutdown(context)
	}()
	slog.Info("舞台吊挂演出就绪验证台已启动", "addr", address, "dataDir", dataDir)
	err = server.ListenAndServe()
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

func runSelfcheck(address string) error {
	temporary, err := os.MkdirTemp("", "stageready-selfcheck-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(temporary)
	store, err := journal.Open(filepath.Join(temporary, "journal"))
	if err != nil {
		return err
	}
	service, err := application.NewService(store, func() time.Time { return time.Date(2026, 8, 21, 10, 0, 0, 0, time.UTC) })
	if err != nil {
		_ = store.Close()
		return err
	}
	defer service.Close()
	server := &http.Server{Addr: address, Handler: httpui.New(service).Handler(), ReadHeaderTimeout: 5 * time.Second}
	serverErrors := make(chan error, 1)
	go func() { serverErrors <- server.ListenAndServe() }()
	client := &http.Client{Timeout: 3 * time.Second}
	base := "http://" + address
	if err := waitForHealth(client, base+"/healthz"); err != nil {
		_ = server.Close()
		return err
	}
	var created struct {
		Detail struct {
			Session struct {
				ID      string `json:"id"`
				Version uint64 `json:"version"`
			} `json:"session"`
		} `json:"detail"`
	}
	if err := requestJSON(client, http.MethodPost, base+"/api/sessions", map[string]any{"productionName": "自检演出", "venue": "黑匣子剧场", "performanceDate": "2026-08-21T00:00:00Z", "technicalDirector": "自检负责人", "expectedVersion": 0, "idempotencyKey": "selfcheck-create"}, &created); err != nil {
		_ = server.Close()
		return err
	}
	sessionID := created.Detail.Session.ID
	version := created.Detail.Session.Version
	request := func(path string, body map[string]any, target any) error {
		body["expectedVersion"] = version
		body["idempotencyKey"] = "selfcheck-" + strconv.Itoa(int(version+1))
		if err := requestJSON(client, http.MethodPost, base+path, body, target); err != nil {
			return err
		}
		var response struct {
			Detail struct {
				Session struct {
					Version uint64 `json:"version"`
				} `json:"session"`
			} `json:"detail"`
		}
		encoded, _ := jsonMarshal(target)
		_ = jsonUnmarshal(encoded, &response)
		if response.Detail.Session.Version > version {
			version = response.Detail.Session.Version
		}
		return nil
	}
	var result any
	if err := request("/api/sessions/"+sessionID+"/devices", map[string]any{"id": "dev-hoist", "name": "1# 电动葫芦", "deviceType": "电动葫芦", "ratedLoadKg": 500, "safeZone": "舞台上空 A 区", "emergencyStopRequired": true}, &result); err != nil {
		_ = server.Close()
		return err
	}
	if err := request("/api/sessions/"+sessionID+"/cues", map[string]any{"id": "cue-rise", "sequence": 1, "deviceID": "dev-hoist", "action": "吊杆上升至定位线", "expectedLoadKg": 300, "minimumClearanceCm": 80, "maximumStopMs": 500}, &result); err != nil {
		_ = server.Close()
		return err
	}
	if err := request("/api/sessions/"+sessionID+"/prepare", map[string]any{}, &result); err != nil {
		_ = server.Close()
		return err
	}
	if err := request("/api/sessions/"+sessionID+"/run", map[string]any{}, &result); err != nil {
		_ = server.Close()
		return err
	}
	if err := request("/api/sessions/"+sessionID+"/attempts", map[string]any{"id": "att-rise", "cueID": "cue-rise", "measuredLoadKg": 280, "measuredClearanceCm": 95, "measuredStopMs": 420, "operator": "自检操作员", "evidenceNote": "急停按钮与标识现场确认"}, &result); err != nil {
		_ = server.Close()
		return err
	}
	if err := request("/api/sessions/"+sessionID+"/reviews", map[string]any{"id": "review-final", "reviewer": "自检检查员", "decision": "Approved", "findings": []string{}}, &result); err != nil {
		_ = server.Close()
		return err
	}
	if err := request("/api/sessions/"+sessionID+"/certificate", map[string]any{"id": "cert-selfcheck"}, &result); err != nil {
		_ = server.Close()
		return err
	}
	var detail struct {
		Session struct {
			Status string `json:"status"`
		} `json:"session"`
		CertificateValid bool `json:"certificateValid"`
	}
	if err := requestJSON(client, http.MethodGet, base+"/api/sessions/"+sessionID, nil, &detail); err != nil {
		_ = server.Close()
		return err
	}
	if detail.Session.Status != "Certified" || !detail.CertificateValid {
		_ = server.Close()
		return fmt.Errorf("selfcheck 未得到 Certified 且 digest 有效的结果")
	}
	if err := server.Close(); err != nil {
		return err
	}
	select {
	case err := <-serverErrors:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			return err
		}
	case <-time.After(2 * time.Second):
		return errors.New("selfcheck 服务未能及时关闭")
	}
	slog.Info("selfcheck 完成", "sessionID", sessionID, "status", detail.Session.Status)
	return nil
}

func waitForHealth(client *http.Client, url string) error {
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		response, err := client.Get(url)
		if err == nil {
			response.Body.Close()
			if response.StatusCode == http.StatusOK {
				return nil
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	return errors.New("selfcheck 服务未能在限定时间内启动")
}

func requestJSON(client *http.Client, method, url string, payload any, target any) error {
	var body io.Reader
	if payload != nil {
		encoded, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		body = strings.NewReader(string(encoded))
	}
	request, err := http.NewRequest(method, url, body)
	if err != nil {
		return err
	}
	if payload != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	response, err := client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		encoded, _ := io.ReadAll(response.Body)
		return fmt.Errorf("%s %s: %s", method, url, strings.TrimSpace(string(encoded)))
	}
	if target == nil {
		return nil
	}
	return json.NewDecoder(response.Body).Decode(target)
}

func jsonMarshal(value any) ([]byte, error)       { return json.Marshal(value) }
func jsonUnmarshal(data []byte, target any) error { return json.Unmarshal(data, target) }

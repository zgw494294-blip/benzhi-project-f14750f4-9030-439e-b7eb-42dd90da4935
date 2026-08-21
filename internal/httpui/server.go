package httpui

import (
	"io/fs"
	"net/http"
	"time"

	"stageready/internal/application"
)

type Server struct {
	application *application.Service
	handler     http.Handler
}

func New(service *application.Service) *Server {
	server := &Server{application: service}
	mux := http.NewServeMux()
	server.registerRoutes(mux)
	server.handler = securityHeaders(mux)
	return server
}

func (s *Server) Handler() http.Handler { return s.handler }

func (s *Server) registerRoutes(mux *http.ServeMux) {
	static, _ := fs.Sub(assets, "assets")
	mux.Handle("GET /assets/", http.StripPrefix("/assets/", http.FileServer(http.FS(static))))
	mux.HandleFunc("GET /", s.HandleRoot)
	mux.HandleFunc("GET /sessions", s.HandleWorkbench)
	mux.HandleFunc("GET /healthz", s.HandleHealth)
	mux.HandleFunc("GET /api/sessions", s.HandleSessionList)
	mux.HandleFunc("POST /api/sessions", s.HandleCreateSession)
	mux.HandleFunc("GET /api/sessions/{id}", s.HandleSessionDetail)
	mux.HandleFunc("POST /api/sessions/{id}/devices", s.HandleAddDevice)
	mux.HandleFunc("PUT /api/sessions/{id}/devices/{deviceID}", s.HandleUpdateDevice)
	mux.HandleFunc("DELETE /api/sessions/{id}/devices/{deviceID}", s.HandleDeleteDevice)
	mux.HandleFunc("POST /api/sessions/{id}/cues", s.HandleAddCue)
	mux.HandleFunc("PUT /api/sessions/{id}/cues/{cueID}", s.HandleUpdateCue)
	mux.HandleFunc("DELETE /api/sessions/{id}/cues/{cueID}", s.HandleDeleteCue)
	mux.HandleFunc("POST /api/sessions/{id}/cues/reorder", s.HandleReorderCues)
	mux.HandleFunc("POST /api/sessions/{id}/configuration/preflight", s.HandleConfigurationPreflight)
	mux.HandleFunc("POST /api/sessions/{id}/configuration/batch", s.HandleConfigurationBatch)
	mux.HandleFunc("POST /api/sessions/{id}/prepare", s.HandlePrepare)
	mux.HandleFunc("POST /api/sessions/{id}/run", s.HandleStartRun)
	mux.HandleFunc("POST /api/sessions/{id}/attempts", s.HandleRecordAttempt)
	mux.HandleFunc("POST /api/sessions/{id}/attempts/batch", s.HandleRecordAttemptBatch)
	mux.HandleFunc("POST /api/sessions/{id}/reviews", s.HandleCompleteReview)
	mux.HandleFunc("POST /api/sessions/{id}/corrections", s.HandleSubmitCorrection)
	mux.HandleFunc("PUT /api/sessions/{id}/corrections/{cueID}", s.HandleUpdateCorrectionTask)
	mux.HandleFunc("POST /api/sessions/{id}/certificate", s.HandleIssueCertificate)
	mux.HandleFunc("GET /api/sessions/{id}/certificates/{certificateID}/verification", s.HandleCertificateVerification)
}

func (s *Server) HandleRoot(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "页面不存在")
		return
	}
	http.Redirect(w, r, "/sessions", http.StatusTemporaryRedirect)
}

func (s *Server) HandleWorkbench(w http.ResponseWriter, _ *http.Request) {
	page, err := assets.ReadFile("assets/index.html")
	if err != nil {
		writeError(w, http.StatusInternalServerError, "ASSET_ERROR", "工作台资源不可用")
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(page)
}

func (s *Server) HandleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "time": time.Now().UTC()})
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self'; connect-src 'self'; object-src 'none'; base-uri 'none'; frame-ancestors 'none'")
		next.ServeHTTP(w, r)
	})
}

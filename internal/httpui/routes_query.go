package httpui

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"stageready/internal/application"
	"stageready/internal/domain"
)

func (s *Server) HandleSessionList(w http.ResponseWriter, r *http.Request) {
	query, err := parseSessionQuery(r)
	if err != nil {
		handleError(w, err)
		return
	}
	page, err := s.application.QuerySessions(query)
	if err != nil {
		handleError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, page)
}

func parseSessionQuery(r *http.Request) (application.SessionQuery, error) {
	values := r.URL.Query()
	allowed := map[string]bool{"status": true, "venue": true, "technicalDirector": true, "performanceFrom": true, "performanceTo": true, "q": true, "sort": true, "order": true, "page": true, "pageSize": true}
	for field := range values {
		if !allowed[field] {
			return application.SessionQuery{}, &application.RequestError{Code: "INVALID_QUERY_FIELD", Message: "未知查询字段 " + field, Field: field}
		}
	}
	query := application.SessionQuery{Venue: values.Get("venue"), TechnicalDirector: values.Get("technicalDirector"), ProductionQuery: values.Get("q"), Sort: values.Get("sort"), Order: values.Get("order")}
	for _, value := range values["status"] {
		for _, status := range strings.Split(value, ",") {
			if status = strings.TrimSpace(status); status != "" {
				query.Statuses = append(query.Statuses, domain.SessionStatus(status))
			}
		}
	}
	parseDate := func(field string) (*time.Time, error) {
		value := strings.TrimSpace(values.Get(field))
		if value == "" {
			return nil, nil
		}
		date, err := time.Parse("2006-01-02", value)
		if err != nil {
			return nil, &application.RequestError{Code: "INVALID_DATE", Message: field + " 必须使用 YYYY-MM-DD", Field: field}
		}
		return &date, nil
	}
	var err error
	if query.PerformanceFrom, err = parseDate("performanceFrom"); err != nil {
		return application.SessionQuery{}, err
	}
	if query.PerformanceTo, err = parseDate("performanceTo"); err != nil {
		return application.SessionQuery{}, err
	}
	parsePositive := func(field string) (int, error) {
		value := strings.TrimSpace(values.Get(field))
		if value == "" {
			return 0, nil
		}
		number, err := strconv.Atoi(value)
		if err != nil {
			return 0, &application.RequestError{Code: "INVALID_NUMBER", Message: field + " 必须是整数", Field: field}
		}
		return number, nil
	}
	if query.Page, err = parsePositive("page"); err != nil {
		return application.SessionQuery{}, err
	}
	if query.PageSize, err = parsePositive("pageSize"); err != nil {
		return application.SessionQuery{}, err
	}
	return query, nil
}

func (s *Server) HandleCertificateVerification(w http.ResponseWriter, r *http.Request) {
	values := r.URL.Query()
	allowed := map[string]bool{"digest": true, "eventHeadHash": true, "sessionVersion": true}
	for field := range values {
		if !allowed[field] {
			handleError(w, &application.RequestError{Code: "INVALID_QUERY_FIELD", Message: "未知查询字段 " + field, Field: field})
			return
		}
	}
	comparison := application.CertificateComparison{}
	if values.Has("digest") {
		value := values.Get("digest")
		comparison.Digest = &value
	}
	if values.Has("eventHeadHash") {
		value := values.Get("eventHeadHash")
		comparison.EventHeadHash = &value
	}
	if values.Has("sessionVersion") {
		value, err := strconv.ParseUint(values.Get("sessionVersion"), 10, 64)
		if err != nil {
			handleError(w, &application.RequestError{Code: "INVALID_SESSION_VERSION", Message: "sessionVersion 必须是非负整数", Field: "sessionVersion"})
			return
		}
		comparison.SessionVersion = &value
	}
	report, err := s.application.VerifyCertificate(r.PathValue("id"), r.PathValue("certificateID"), comparison)
	if err != nil {
		handleError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, report)
}

func (s *Server) HandleSessionDetail(w http.ResponseWriter, r *http.Request) {
	detail, err := s.application.GetSession(r.PathValue("id"))
	if err != nil {
		handleError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, detail)
}

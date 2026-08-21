package httpui

import (
	"net/http"

	"stageready/internal/application"
)

func (s *Server) HandleRecordAttempt(w http.ResponseWriter, r *http.Request) {
	var command application.RecordAttemptCommand
	if err := decodeJSON(w, r, &command); err != nil {
		handleError(w, err)
		return
	}
	command.SessionID = r.PathValue("id")
	result, err := s.application.RecordAttempt(command)
	if err != nil {
		handleError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) HandleRecordAttemptBatch(w http.ResponseWriter, r *http.Request) {
	var command application.RecordAttemptBatchCommand
	if err := decodeJSON(w, r, &command); err != nil {
		handleError(w, err)
		return
	}
	command.SessionID = r.PathValue("id")
	result, err := s.application.RecordAttemptBatch(command)
	if err != nil {
		handleError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) HandleCompleteReview(w http.ResponseWriter, r *http.Request) {
	var command application.CompleteReviewCommand
	if err := decodeJSON(w, r, &command); err != nil {
		handleError(w, err)
		return
	}
	command.SessionID = r.PathValue("id")
	result, err := s.application.CompleteReview(command)
	if err != nil {
		handleError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) HandleSubmitCorrection(w http.ResponseWriter, r *http.Request) {
	var command application.SubmitCorrectionCommand
	if err := decodeJSON(w, r, &command); err != nil {
		handleError(w, err)
		return
	}
	command.SessionID = r.PathValue("id")
	result, err := s.application.SubmitCorrection(command)
	if err != nil {
		handleError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) HandleUpdateCorrectionTask(w http.ResponseWriter, r *http.Request) {
	var command application.UpdateCorrectionTaskCommand
	if err := decodeJSON(w, r, &command); err != nil {
		handleError(w, err)
		return
	}
	command.SessionID, command.CueID = r.PathValue("id"), r.PathValue("cueID")
	result, err := s.application.UpdateCorrectionTask(command)
	if err != nil {
		handleError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) HandleIssueCertificate(w http.ResponseWriter, r *http.Request) {
	var command application.IssueCertificateCommand
	if err := decodeJSON(w, r, &command); err != nil {
		handleError(w, err)
		return
	}
	command.SessionID = r.PathValue("id")
	result, err := s.application.IssueCertificate(command)
	if err != nil {
		handleError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

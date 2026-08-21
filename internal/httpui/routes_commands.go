package httpui

import (
	"net/http"

	"stageready/internal/application"
)

func (s *Server) HandleCreateSession(w http.ResponseWriter, r *http.Request) {
	var command application.CreateSessionCommand
	if err := decodeJSON(w, r, &command); err != nil {
		handleError(w, err)
		return
	}
	result, err := s.application.CreateSessionContext(r.Context(), command)
	if err != nil {
		handleError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, result)
}

func (s *Server) HandleAddDevice(w http.ResponseWriter, r *http.Request) {
	var command application.AddDeviceCommand
	if err := decodeJSON(w, r, &command); err != nil {
		handleError(w, err)
		return
	}
	command.SessionID = r.PathValue("id")
	result, err := s.application.AddDevice(command)
	if err != nil {
		handleError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) HandleUpdateDevice(w http.ResponseWriter, r *http.Request) {
	var command application.UpdateDeviceCommand
	if err := decodeJSON(w, r, &command); err != nil {
		handleError(w, err)
		return
	}
	command.SessionID, command.ID = r.PathValue("id"), r.PathValue("deviceID")
	result, err := s.application.UpdateDevice(command)
	if err != nil {
		handleError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) HandleDeleteDevice(w http.ResponseWriter, r *http.Request) {
	var command application.DeleteDeviceCommand
	if err := decodeJSON(w, r, &command); err != nil {
		handleError(w, err)
		return
	}
	command.SessionID, command.DeviceID = r.PathValue("id"), r.PathValue("deviceID")
	result, err := s.application.DeleteDevice(command)
	if err != nil {
		handleError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) HandleAddCue(w http.ResponseWriter, r *http.Request) {
	var command application.AddCueCommand
	if err := decodeJSON(w, r, &command); err != nil {
		handleError(w, err)
		return
	}
	command.SessionID = r.PathValue("id")
	result, err := s.application.AddCue(command)
	if err != nil {
		handleError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) HandleUpdateCue(w http.ResponseWriter, r *http.Request) {
	var command application.UpdateCueCommand
	if err := decodeJSON(w, r, &command); err != nil {
		handleError(w, err)
		return
	}
	command.SessionID, command.ID = r.PathValue("id"), r.PathValue("cueID")
	result, err := s.application.UpdateCue(command)
	if err != nil {
		handleError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) HandleDeleteCue(w http.ResponseWriter, r *http.Request) {
	var command application.DeleteCueCommand
	if err := decodeJSON(w, r, &command); err != nil {
		handleError(w, err)
		return
	}
	command.SessionID, command.CueID = r.PathValue("id"), r.PathValue("cueID")
	result, err := s.application.DeleteCue(command)
	if err != nil {
		handleError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) HandleReorderCues(w http.ResponseWriter, r *http.Request) {
	var command application.ReorderCuesCommand
	if err := decodeJSON(w, r, &command); err != nil {
		handleError(w, err)
		return
	}
	command.SessionID = r.PathValue("id")
	result, err := s.application.ReorderCues(command)
	if err != nil {
		handleError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) HandleConfigurationPreflight(w http.ResponseWriter, r *http.Request) {
	var input application.ConfigurationPreflightInput
	if err := decodeJSON(w, r, &input); err != nil {
		handleError(w, err)
		return
	}
	report, err := s.application.PreflightConfiguration(r.PathValue("id"), input)
	if err != nil {
		handleError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, report)
}

func (s *Server) HandleConfigurationBatch(w http.ResponseWriter, r *http.Request) {
	var command application.ConfigurationBatchCommand
	if err := decodeJSON(w, r, &command); err != nil {
		handleError(w, err)
		return
	}
	command.SessionID = r.PathValue("id")
	result, err := s.application.ConfirmConfigurationBatch(command)
	if err != nil {
		handleError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) HandlePrepare(w http.ResponseWriter, r *http.Request) {
	s.handleSessionCommand(w, r, s.application.Prepare)
}

func (s *Server) HandleStartRun(w http.ResponseWriter, r *http.Request) {
	s.handleSessionCommand(w, r, s.application.StartRun)
}

func (s *Server) handleSessionCommand(w http.ResponseWriter, r *http.Request, action func(application.SessionCommand) (application.CommandResult, error)) {
	var command application.SessionCommand
	if err := decodeJSON(w, r, &command); err != nil {
		handleError(w, err)
		return
	}
	command.SessionID = r.PathValue("id")
	result, err := action(command)
	if err != nil {
		handleError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

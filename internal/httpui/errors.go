package httpui

import (
	"errors"
	"net/http"

	"stageready/internal/application"
	"stageready/internal/domain"
	"stageready/internal/journal"
)

type errorResponse struct {
	Error struct {
		Code     string                   `json:"code"`
		Message  string                   `json:"message"`
		Field    string                   `json:"field,omitempty"`
		Problems []domain.ValidationIssue `json:"problems,omitempty"`
	} `json:"error"`
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	response := errorResponse{}
	response.Error.Code = code
	response.Error.Message = message
	writeJSON(w, status, response)
}

func handleError(w http.ResponseWriter, err error) {
	var decode *requestDecodeError
	var request *application.RequestError
	var rule *domain.RuleError
	var validation *domain.ValidationError
	var conflict *journal.ConflictError
	var idempotency *journal.IdempotencyError
	var notFound *application.NotFoundError
	switch {
	case errors.As(err, &decode):
		writeError(w, http.StatusBadRequest, "INVALID_JSON", decode.Message)
	case errors.As(err, &request):
		response := errorResponse{}
		response.Error.Code, response.Error.Message, response.Error.Field = request.Code, request.Message, request.Field
		writeJSON(w, http.StatusBadRequest, response)
	case errors.As(err, &validation):
		response := errorResponse{}
		response.Error.Code, response.Error.Message, response.Error.Problems = validation.Code, validation.Message, validation.Problems
		writeJSON(w, http.StatusUnprocessableEntity, response)
	case errors.As(err, &rule):
		writeError(w, http.StatusUnprocessableEntity, rule.Code, rule.Message)
	case errors.As(err, &conflict):
		writeError(w, http.StatusConflict, "VERSION_CONFLICT", "会话版本已变化，请刷新后重试")
	case errors.As(err, &idempotency):
		writeError(w, http.StatusConflict, "IDEMPOTENCY_CONFLICT", "幂等键已用于其他命令")
	case errors.As(err, &notFound):
		message := "验证会话不存在"
		if notFound.Resource == "certificate" {
			message = "不可变证书不存在"
		}
		writeError(w, http.StatusNotFound, "NOT_FOUND", message)
	default:
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "服务处理请求时发生错误")
	}
}

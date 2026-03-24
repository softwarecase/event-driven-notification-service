package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/softwarecase/event-driven-notification-service/internal/domain"
)

type ErrorResponse struct {
	Error   string `json:"error"`
	Details string `json:"details,omitempty"`
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, domain.ErrNotFound):
		writeJSON(w, http.StatusNotFound, ErrorResponse{Error: err.Error()})
	case errors.Is(err, domain.ErrCannotCancel):
		writeJSON(w, http.StatusConflict, ErrorResponse{Error: err.Error()})
	case errors.Is(err, domain.ErrBatchTooLarge):
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: err.Error()})
	case errors.Is(err, domain.ErrInvalidChannel),
		errors.Is(err, domain.ErrInvalidPriority),
		errors.Is(err, domain.ErrEmptyRecipient),
		errors.Is(err, domain.ErrEmptyContent),
		errors.Is(err, domain.ErrContentTooLong),
		errors.Is(err, domain.ErrScheduleInPast),
		errors.Is(err, domain.ErrMissingTemplateVars),
		errors.Is(err, domain.ErrTemplateInactive):
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: err.Error()})
	case errors.Is(err, domain.ErrDuplicateIDKey):
		writeJSON(w, http.StatusConflict, ErrorResponse{Error: err.Error()})
	default:
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "internal server error"})
	}
}

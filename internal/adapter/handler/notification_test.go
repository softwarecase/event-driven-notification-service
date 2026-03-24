package handler

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/softwarecase/event-driven-notification-service/internal/domain"
	"github.com/stretchr/testify/assert"
)

func TestWriteError(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		statusCode int
	}{
		{"not found", domain.ErrNotFound, http.StatusNotFound},
		{"cannot cancel", domain.ErrCannotCancel, http.StatusConflict},
		{"batch too large", domain.ErrBatchTooLarge, http.StatusBadRequest},
		{"invalid channel", domain.ErrInvalidChannel, http.StatusBadRequest},
		{"empty content", domain.ErrEmptyContent, http.StatusBadRequest},
		{"content too long", domain.ErrContentTooLong, http.StatusBadRequest},
		{"schedule in past", domain.ErrScheduleInPast, http.StatusBadRequest},
		{"duplicate key", domain.ErrDuplicateIDKey, http.StatusConflict},
		{"unknown error", errors.New("something"), http.StatusInternalServerError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			writeError(w, tt.err)
			assert.Equal(t, tt.statusCode, w.Code)
		})
	}
}

package handler

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/softwarecase/event-driven-notification-service/internal/domain"
	"github.com/softwarecase/event-driven-notification-service/internal/port"
	"github.com/softwarecase/event-driven-notification-service/internal/service"
)

type NotificationHandler struct {
	svc *service.NotificationService
}

func NewNotificationHandler(svc *service.NotificationService) *NotificationHandler {
	return &NotificationHandler{svc: svc}
}

func (h *NotificationHandler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Post("/", h.Create)
	r.Post("/batch", h.CreateBatch)
	r.Get("/", h.List)
	r.Get("/{id}", h.GetByID)
	r.Get("/batch/{batchId}", h.GetByBatchID)
	r.Patch("/{id}/cancel", h.Cancel)
	r.Patch("/batch/{batchId}/cancel", h.CancelBatch)
	return r
}

func (h *NotificationHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req service.CreateNotificationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "invalid request body"})
		return
	}

	if req.Recipient == "" {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: domain.ErrEmptyRecipient.Error()})
		return
	}

	notification, err := h.svc.Create(r.Context(), req)
	if err != nil {
		writeError(w, err)
		return
	}

	writeJSON(w, http.StatusAccepted, notification)
}

func (h *NotificationHandler) CreateBatch(w http.ResponseWriter, r *http.Request) {
	var requests struct {
		Notifications []service.CreateNotificationRequest `json:"notifications"`
	}
	if err := json.NewDecoder(r.Body).Decode(&requests); err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "invalid request body"})
		return
	}

	if len(requests.Notifications) == 0 {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "notifications array is required"})
		return
	}

	result, err := h.svc.CreateBatch(r.Context(), requests.Notifications)
	if err != nil {
		writeError(w, err)
		return
	}

	writeJSON(w, http.StatusAccepted, result)
}

func (h *NotificationHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "invalid notification ID"})
		return
	}

	notification, err := h.svc.GetByID(r.Context(), id)
	if err != nil {
		writeError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, notification)
}

func (h *NotificationHandler) GetByBatchID(w http.ResponseWriter, r *http.Request) {
	batchID, err := uuid.Parse(chi.URLParam(r, "batchId"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "invalid batch ID"})
		return
	}

	notifications, err := h.svc.GetByBatchID(r.Context(), batchID)
	if err != nil {
		writeError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"batch_id":      batchID,
		"count":         len(notifications),
		"notifications": notifications,
	})
}

func (h *NotificationHandler) Cancel(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "invalid notification ID"})
		return
	}

	if err := h.svc.Cancel(r.Context(), id); err != nil {
		writeError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "cancelled"})
}

func (h *NotificationHandler) CancelBatch(w http.ResponseWriter, r *http.Request) {
	batchID, err := uuid.Parse(chi.URLParam(r, "batchId"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "invalid batch ID"})
		return
	}

	count, err := h.svc.CancelBatch(r.Context(), batchID)
	if err != nil {
		writeError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":   "cancelled",
		"affected": count,
	})
}

func (h *NotificationHandler) List(w http.ResponseWriter, r *http.Request) {
	filter := port.NotificationFilter{
		Page:     1,
		PageSize: 20,
	}

	if p := r.URL.Query().Get("page"); p != "" {
		if v, err := strconv.Atoi(p); err == nil {
			filter.Page = v
		}
	}
	if ps := r.URL.Query().Get("page_size"); ps != "" {
		if v, err := strconv.Atoi(ps); err == nil {
			filter.PageSize = v
		}
	}
	if s := r.URL.Query().Get("status"); s != "" {
		status := domain.Status(s)
		filter.Status = &status
	}
	if c := r.URL.Query().Get("channel"); c != "" {
		channel := domain.Channel(c)
		filter.Channel = &channel
	}
	if b := r.URL.Query().Get("batch_id"); b != "" {
		if id, err := uuid.Parse(b); err == nil {
			filter.BatchID = &id
		}
	}
	if from := r.URL.Query().Get("from"); from != "" {
		if t, err := time.Parse(time.RFC3339, from); err == nil {
			filter.FromDate = &t
		}
	}
	if to := r.URL.Query().Get("to"); to != "" {
		if t, err := time.Parse(time.RFC3339, to); err == nil {
			filter.ToDate = &t
		}
	}

	result, err := h.svc.List(r.Context(), filter)
	if err != nil {
		writeError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, result)
}

package service

import (
	"context"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/softwarecase/event-driven-notification-service/internal/domain"
	"github.com/softwarecase/event-driven-notification-service/internal/port"
)

type NotificationService struct {
	repo        port.NotificationRepository
	templateSvc *TemplateService
	queue       port.QueuePublisher
	event       port.EventPublisher
	logger      *slog.Logger
}

func NewNotificationService(
	repo port.NotificationRepository,
	templateSvc *TemplateService,
	queue port.QueuePublisher,
	event port.EventPublisher,
	logger *slog.Logger,
) *NotificationService {
	return &NotificationService{
		repo:        repo,
		templateSvc: templateSvc,
		queue:       queue,
		event:       event,
		logger:      logger,
	}
}

type CreateNotificationRequest struct {
	Channel        string                 `json:"channel" validate:"required,oneof=sms email push"`
	Recipient      string                 `json:"recipient" validate:"required"`
	Subject        string                 `json:"subject,omitempty"`
	Content        string                 `json:"content" validate:"required_without=TemplateID"`
	Priority       string                 `json:"priority,omitempty" validate:"omitempty,oneof=high normal low"`
	ScheduledAt    *time.Time             `json:"scheduled_at,omitempty"`
	TemplateID     *uuid.UUID             `json:"template_id,omitempty"`
	TemplateVars   map[string]interface{} `json:"template_vars,omitempty"`
	IdempotencyKey *string                `json:"idempotency_key,omitempty"`
	Metadata       map[string]interface{} `json:"metadata,omitempty"`
}

func (s *NotificationService) Create(ctx context.Context, req CreateNotificationRequest) (*domain.Notification, error) {
	channel := domain.Channel(req.Channel)
	if !channel.IsValid() {
		return nil, domain.ErrInvalidChannel
	}

	// Render template if provided
	if req.TemplateID != nil && s.templateSvc != nil {
		tmpl, err := s.templateSvc.GetByID(ctx, *req.TemplateID)
		if err != nil {
			return nil, err
		}
		if !tmpl.Active {
			return nil, domain.ErrTemplateInactive
		}
		subject, content, err := s.templateSvc.Render(ctx, tmpl, req.TemplateVars)
		if err != nil {
			return nil, err
		}
		req.Content = content
		if subject != "" {
			req.Subject = subject
		}
	}

	if err := validateContent(channel, req.Content); err != nil {
		return nil, err
	}

	// Idempotency check
	if req.IdempotencyKey != nil {
		existing, err := s.repo.GetByIdempotencyKey(ctx, *req.IdempotencyKey)
		if err != nil {
			return nil, err
		}
		if existing != nil {
			return existing, nil
		}
	}

	priority := domain.PriorityFromString(req.Priority)
	n := domain.NewNotification(channel, req.Recipient, req.Content, priority)
	n.Subject = req.Subject
	n.IdempotencyKey = req.IdempotencyKey
	n.TemplateID = req.TemplateID
	n.TemplateVars = req.TemplateVars

	if req.Metadata != nil {
		n.Metadata = req.Metadata
	}

	// Handle scheduling
	if req.ScheduledAt != nil {
		if req.ScheduledAt.Before(time.Now().UTC()) {
			return nil, domain.ErrScheduleInPast
		}
		n.ScheduledAt = req.ScheduledAt
		n.Status = domain.StatusScheduled
	}

	if err := s.repo.Create(ctx, n); err != nil {
		return nil, err
	}

	// Enqueue if not scheduled
	if n.Status == domain.StatusPending {
		n.Status = domain.StatusQueued
		if err := s.repo.UpdateStatus(ctx, n.ID, domain.StatusQueued); err != nil {
			s.logger.Error("failed to update status to queued", "id", n.ID, "error", err)
		}
		if err := s.queue.Enqueue(ctx, port.QueueMessage{
			NotificationID: n.ID.String(),
			Channel:        string(n.Channel),
			Priority:       int(n.Priority),
		}); err != nil {
			s.logger.Error("failed to enqueue notification", "id", n.ID, "error", err)
		}
	}

	s.event.Publish(port.StatusEvent{
		NotificationID: n.ID.String(),
		Status:         string(n.Status),
		Channel:        string(n.Channel),
		Timestamp:      n.CreatedAt.Format(time.RFC3339),
	})

	return n, nil
}

func (s *NotificationService) CreateBatch(ctx context.Context, requests []CreateNotificationRequest) (*domain.BatchResult, error) {
	if len(requests) > domain.MaxBatchSize {
		return nil, domain.ErrBatchTooLarge
	}

	batchID := uuid.New()
	result := &domain.BatchResult{
		BatchID: batchID,
		Total:   len(requests),
	}

	var notifications []*domain.Notification
	var queueMsgs []port.QueueMessage

	for i, req := range requests {
		channel := domain.Channel(req.Channel)
		if !channel.IsValid() {
			result.Rejected++
			result.Errors = append(result.Errors, domain.BatchError{Index: i, Message: "invalid channel"})
			continue
		}

		if err := validateContent(channel, req.Content); err != nil {
			result.Rejected++
			result.Errors = append(result.Errors, domain.BatchError{Index: i, Message: err.Error()})
			continue
		}

		priority := domain.PriorityFromString(req.Priority)
		n := domain.NewNotification(channel, req.Recipient, req.Content, priority)
		n.BatchID = &batchID
		n.Subject = req.Subject
		n.IdempotencyKey = req.IdempotencyKey
		n.TemplateID = req.TemplateID
		n.TemplateVars = req.TemplateVars
		if req.Metadata != nil {
			n.Metadata = req.Metadata
		}

		if req.ScheduledAt != nil && req.ScheduledAt.After(time.Now().UTC()) {
			n.ScheduledAt = req.ScheduledAt
			n.Status = domain.StatusScheduled
		} else {
			n.Status = domain.StatusQueued
			queueMsgs = append(queueMsgs, port.QueueMessage{
				NotificationID: n.ID.String(),
				Channel:        string(n.Channel),
				Priority:       int(n.Priority),
			})
		}

		notifications = append(notifications, n)
		result.Accepted++
		result.Notifications = append(result.Notifications, n.ID)
	}

	if len(notifications) > 0 {
		if err := s.repo.CreateBatch(ctx, notifications); err != nil {
			return nil, err
		}

		if len(queueMsgs) > 0 {
			if err := s.queue.EnqueueBatch(ctx, queueMsgs); err != nil {
				s.logger.Error("failed to enqueue batch", "batch_id", batchID, "error", err)
			}
		}
	}

	return result, nil
}

func (s *NotificationService) GetByID(ctx context.Context, id uuid.UUID) (*domain.Notification, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *NotificationService) GetByBatchID(ctx context.Context, batchID uuid.UUID) ([]*domain.Notification, error) {
	return s.repo.GetByBatchID(ctx, batchID)
}

func (s *NotificationService) Cancel(ctx context.Context, id uuid.UUID) error {
	err := s.repo.CancelByID(ctx, id)
	if err != nil {
		return err
	}
	s.event.Publish(port.StatusEvent{
		NotificationID: id.String(),
		Status:         string(domain.StatusCancelled),
		Timestamp:      time.Now().UTC().Format(time.RFC3339),
	})
	return nil
}

func (s *NotificationService) CancelBatch(ctx context.Context, batchID uuid.UUID) (int64, error) {
	return s.repo.CancelByBatchID(ctx, batchID)
}

func (s *NotificationService) List(ctx context.Context, filter port.NotificationFilter) (*port.PaginatedResult, error) {
	return s.repo.List(ctx, filter)
}

func validateContent(channel domain.Channel, content string) error {
	if content == "" {
		return domain.ErrEmptyContent
	}
	switch channel {
	case domain.ChannelSMS:
		if len(content) > 1600 { // Multiple SMS segments
			return domain.ErrContentTooLong
		}
	case domain.ChannelPush:
		if len(content) > 4096 {
			return domain.ErrContentTooLong
		}
	case domain.ChannelEmail:
		if len(content) > 100000 {
			return domain.ErrContentTooLong
		}
	}
	return nil
}

package port

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/softwarecase/event-driven-notification-service/internal/domain"
)

type NotificationFilter struct {
	Status    *domain.Status
	Channel   *domain.Channel
	BatchID   *uuid.UUID
	FromDate  *time.Time
	ToDate    *time.Time
	Page      int
	PageSize  int
}

type PaginatedResult struct {
	Data       []*domain.Notification `json:"data"`
	Page       int                    `json:"page"`
	PageSize   int                    `json:"page_size"`
	Total      int64                  `json:"total"`
	TotalPages int                    `json:"total_pages"`
}

type NotificationRepository interface {
	Create(ctx context.Context, n *domain.Notification) error
	CreateBatch(ctx context.Context, notifications []*domain.Notification) error
	GetByID(ctx context.Context, id uuid.UUID) (*domain.Notification, error)
	GetByBatchID(ctx context.Context, batchID uuid.UUID) ([]*domain.Notification, error)
	Update(ctx context.Context, n *domain.Notification) error
	UpdateStatus(ctx context.Context, id uuid.UUID, status domain.Status) error
	CancelByID(ctx context.Context, id uuid.UUID) error
	CancelByBatchID(ctx context.Context, batchID uuid.UUID) (int64, error)
	List(ctx context.Context, filter NotificationFilter) (*PaginatedResult, error)
	GetScheduledReady(ctx context.Context, limit int) ([]*domain.Notification, error)
	GetRetryReady(ctx context.Context, limit int) ([]*domain.Notification, error)
	GetByIdempotencyKey(ctx context.Context, key string) (*domain.Notification, error)
}

type DeliveryAttemptRepository interface {
	Create(ctx context.Context, attempt *domain.DeliveryAttempt) error
	GetByNotificationID(ctx context.Context, notificationID uuid.UUID) ([]*domain.DeliveryAttempt, error)
}

type DeadLetterRepository interface {
	Create(ctx context.Context, entry *domain.DeadLetterEntry) error
	List(ctx context.Context, page, pageSize int) ([]*domain.DeadLetterEntry, error)
	MarkReprocessed(ctx context.Context, id uuid.UUID) error
}

type TemplateRepository interface {
	Create(ctx context.Context, t *domain.Template) error
	GetByID(ctx context.Context, id uuid.UUID) (*domain.Template, error)
	GetByName(ctx context.Context, name string) (*domain.Template, error)
	Update(ctx context.Context, t *domain.Template) error
	Delete(ctx context.Context, id uuid.UUID) error
	List(ctx context.Context, page, pageSize int) ([]*domain.Template, int64, error)
}

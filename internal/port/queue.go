package port

import (
	"context"

	"github.com/softwarecase/event-driven-notification-service/internal/domain"
)

type QueueMessage struct {
	NotificationID string `json:"id"`
	Channel        string `json:"channel"`
	Priority       int    `json:"priority"`
	TraceParent    string `json:"traceparent,omitempty"`
}

type QueuePublisher interface {
	Enqueue(ctx context.Context, msg QueueMessage) error
	EnqueueBatch(ctx context.Context, msgs []QueueMessage) error
}

type QueueConsumer interface {
	Dequeue(ctx context.Context, channel domain.Channel) (*QueueMessage, error)
	Acknowledge(ctx context.Context, channel domain.Channel, notificationID string) error
	QueueDepth(ctx context.Context, channel domain.Channel) (int64, error)
}

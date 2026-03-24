package domain

import (
	"time"

	"github.com/google/uuid"
)

type Channel string

const (
	ChannelSMS   Channel = "sms"
	ChannelEmail Channel = "email"
	ChannelPush  Channel = "push"
)

func (c Channel) IsValid() bool {
	switch c {
	case ChannelSMS, ChannelEmail, ChannelPush:
		return true
	}
	return false
}

type Priority int

const (
	PriorityHigh   Priority = 0
	PriorityNormal Priority = 1
	PriorityLow    Priority = 2
)

func PriorityFromString(s string) Priority {
	switch s {
	case "high":
		return PriorityHigh
	case "low":
		return PriorityLow
	default:
		return PriorityNormal
	}
}

func (p Priority) String() string {
	switch p {
	case PriorityHigh:
		return "high"
	case PriorityLow:
		return "low"
	default:
		return "normal"
	}
}

type Status string

const (
	StatusPending    Status = "pending"
	StatusScheduled  Status = "scheduled"
	StatusQueued     Status = "queued"
	StatusProcessing Status = "processing"
	StatusDelivered  Status = "delivered"
	StatusFailed     Status = "failed"
	StatusCancelled  Status = "cancelled"
)

func (s Status) IsFinal() bool {
	return s == StatusDelivered || s == StatusFailed || s == StatusCancelled
}

func (s Status) CanCancel() bool {
	return s == StatusPending || s == StatusScheduled || s == StatusQueued
}

type Notification struct {
	ID             uuid.UUID              `json:"id"`
	BatchID        *uuid.UUID             `json:"batch_id,omitempty"`
	IdempotencyKey *string                `json:"idempotency_key,omitempty"`
	Channel        Channel                `json:"channel"`
	Recipient      string                 `json:"recipient"`
	Subject        string                 `json:"subject,omitempty"`
	Content        string                 `json:"content"`
	Priority       Priority               `json:"priority"`
	Status         Status                 `json:"status"`
	ScheduledAt    *time.Time             `json:"scheduled_at,omitempty"`
	SentAt         *time.Time             `json:"sent_at,omitempty"`
	ProviderMsgID  *string                `json:"provider_msg_id,omitempty"`
	RetryCount     int                    `json:"retry_count"`
	MaxRetries     int                    `json:"max_retries"`
	NextRetryAt    *time.Time             `json:"next_retry_at,omitempty"`
	Metadata       map[string]interface{} `json:"metadata,omitempty"`
	TemplateID     *uuid.UUID             `json:"template_id,omitempty"`
	TemplateVars   map[string]interface{} `json:"template_vars,omitempty"`
	ErrorMessage   *string                `json:"error_message,omitempty"`
	CreatedAt      time.Time              `json:"created_at"`
	UpdatedAt      time.Time              `json:"updated_at"`
}

func NewNotification(channel Channel, recipient, content string, priority Priority) *Notification {
	now := time.Now().UTC()
	return &Notification{
		ID:         uuid.New(),
		Channel:    channel,
		Recipient:  recipient,
		Content:    content,
		Priority:   priority,
		Status:     StatusPending,
		RetryCount: 0,
		MaxRetries: 3,
		Metadata:   make(map[string]interface{}),
		CreatedAt:  now,
		UpdatedAt:  now,
	}
}

type DeliveryAttempt struct {
	ID             uuid.UUID  `json:"id"`
	NotificationID uuid.UUID  `json:"notification_id"`
	AttemptNumber  int        `json:"attempt_number"`
	Status         string     `json:"status"` // "success" or "failure"
	ProviderMsgID  *string    `json:"provider_msg_id,omitempty"`
	StatusCode     *int       `json:"status_code,omitempty"`
	ResponseBody   *string    `json:"response_body,omitempty"`
	ErrorMessage   *string    `json:"error_message,omitempty"`
	DurationMs     int        `json:"duration_ms"`
	CreatedAt      time.Time  `json:"created_at"`
}

type DeadLetterEntry struct {
	ID             uuid.UUID              `json:"id"`
	NotificationID uuid.UUID              `json:"notification_id"`
	Reason         string                 `json:"reason"`
	LastError      *string                `json:"last_error,omitempty"`
	Payload        map[string]interface{} `json:"payload"`
	CreatedAt      time.Time              `json:"created_at"`
	ReprocessedAt  *time.Time             `json:"reprocessed_at,omitempty"`
}

package service

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"math/rand"
	"time"

	"github.com/google/uuid"
	"github.com/softwarecase/event-driven-notification-service/internal/domain"
	"github.com/softwarecase/event-driven-notification-service/internal/port"
	"github.com/softwarecase/event-driven-notification-service/pkg/circuitbreaker"
	"github.com/softwarecase/event-driven-notification-service/pkg/ratelimit"
)

// MetricsRecorder records delivery metrics for the metrics endpoint.
type MetricsRecorder interface {
	RecordDelivery(latencyMs int64)
	RecordFailure()
}

type DeliveryService struct {
	notifRepo   port.NotificationRepository
	attemptRepo port.DeliveryAttemptRepository
	dlqRepo     port.DeadLetterRepository
	provider    port.DeliveryProvider
	queue       port.QueuePublisher
	event       port.EventPublisher
	limiter     *ratelimit.Limiter
	breakers    map[domain.Channel]*circuitbreaker.CircuitBreaker
	metrics     MetricsRecorder
	logger      *slog.Logger
	maxRetries  int
	baseDelay   time.Duration
	maxDelay    time.Duration
}

type DeliveryConfig struct {
	MaxRetries int
	BaseDelay  time.Duration
	MaxDelay   time.Duration
}

func NewDeliveryService(
	notifRepo port.NotificationRepository,
	attemptRepo port.DeliveryAttemptRepository,
	dlqRepo port.DeadLetterRepository,
	provider port.DeliveryProvider,
	queue port.QueuePublisher,
	event port.EventPublisher,
	limiter *ratelimit.Limiter,
	metrics MetricsRecorder,
	logger *slog.Logger,
	cfg DeliveryConfig,
) *DeliveryService {
	breakers := map[domain.Channel]*circuitbreaker.CircuitBreaker{
		domain.ChannelSMS:   circuitbreaker.New(5, 3, 3, 30*time.Second),
		domain.ChannelEmail: circuitbreaker.New(5, 3, 3, 30*time.Second),
		domain.ChannelPush:  circuitbreaker.New(5, 3, 3, 30*time.Second),
	}

	return &DeliveryService{
		notifRepo:   notifRepo,
		attemptRepo: attemptRepo,
		dlqRepo:     dlqRepo,
		provider:    provider,
		queue:       queue,
		event:       event,
		limiter:     limiter,
		breakers:    breakers,
		metrics:     metrics,
		logger:      logger,
		maxRetries:  cfg.MaxRetries,
		baseDelay:   cfg.BaseDelay,
		maxDelay:    cfg.MaxDelay,
	}
}

func (s *DeliveryService) Process(ctx context.Context, notificationID string) error {
	id, err := uuid.Parse(notificationID)
	if err != nil {
		return fmt.Errorf("parse notification ID: %w", err)
	}

	n, err := s.notifRepo.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("get notification: %w", err)
	}

	// Check if already in final state
	if n.Status.IsFinal() {
		return nil
	}

	// Update status to processing
	n.Status = domain.StatusProcessing
	if err := s.notifRepo.Update(ctx, n); err != nil {
		return fmt.Errorf("update status: %w", err)
	}

	s.publishEvent(n)

	// Check circuit breaker
	breaker := s.breakers[n.Channel]
	if !breaker.Allow() {
		s.logger.Warn("circuit breaker open", "channel", n.Channel, "notification_id", n.ID)
		s.handleRetry(ctx, n, domain.ErrCircuitOpen)
		return domain.ErrCircuitOpen
	}

	// Check rate limit
	allowed, err := s.limiter.Allow(ctx, string(n.Channel))
	if err != nil {
		s.logger.Error("rate limit check failed", "error", err)
		return err
	}
	if !allowed {
		// Re-enqueue with slight delay
		if err := s.queue.Enqueue(ctx, port.QueueMessage{
			NotificationID: n.ID.String(),
			Channel:        string(n.Channel),
			Priority:       int(n.Priority),
		}); err != nil {
			s.logger.Error("failed to re-enqueue rate limited notification", "error", err)
		}
		n.Status = domain.StatusQueued
		_ = s.notifRepo.Update(ctx, n)
		return domain.ErrRateLimited
	}

	// Send via provider
	start := time.Now()
	resp, sendErr := s.provider.Send(ctx, port.SendRequest{
		To:      n.Recipient,
		Channel: string(n.Channel),
		Content: n.Content,
	})
	duration := time.Since(start)

	// Record delivery attempt
	attempt := &domain.DeliveryAttempt{
		ID:             uuid.New(),
		NotificationID: n.ID,
		AttemptNumber:  n.RetryCount + 1,
		DurationMs:     int(duration.Milliseconds()),
		CreatedAt:      time.Now().UTC(),
	}

	if sendErr != nil {
		attempt.Status = "failure"
		errMsg := sendErr.Error()
		attempt.ErrorMessage = &errMsg
		_ = s.attemptRepo.Create(ctx, attempt)

		breaker.RecordFailure()
		s.metrics.RecordFailure()
		s.handleRetry(ctx, n, sendErr)
		return sendErr
	}

	// Success
	s.metrics.RecordDelivery(duration.Milliseconds())
	attempt.Status = "success"
	if resp != nil {
		attempt.ProviderMsgID = &resp.MessageID
	}
	_ = s.attemptRepo.Create(ctx, attempt)

	breaker.RecordSuccess()

	now := time.Now().UTC()
	n.Status = domain.StatusDelivered
	n.SentAt = &now
	if resp != nil {
		n.ProviderMsgID = &resp.MessageID
	}
	if err := s.notifRepo.Update(ctx, n); err != nil {
		s.logger.Error("failed to update delivered notification", "error", err)
	}

	s.publishEvent(n)
	s.logger.Info("notification delivered",
		"notification_id", n.ID,
		"channel", n.Channel,
		"duration_ms", duration.Milliseconds(),
	)

	return nil
}

func (s *DeliveryService) handleRetry(ctx context.Context, n *domain.Notification, sendErr error) {
	n.RetryCount++
	errMsg := sendErr.Error()
	n.ErrorMessage = &errMsg

	if n.RetryCount >= s.maxRetries {
		// Move to DLQ
		n.Status = domain.StatusFailed
		_ = s.notifRepo.Update(ctx, n)

		dlqEntry := &domain.DeadLetterEntry{
			ID:             uuid.New(),
			NotificationID: n.ID,
			Reason:         "max retries exceeded",
			LastError:      &errMsg,
			Payload: map[string]interface{}{
				"channel":   n.Channel,
				"recipient": n.Recipient,
				"content":   n.Content,
			},
			CreatedAt: time.Now().UTC(),
		}
		if err := s.dlqRepo.Create(ctx, dlqEntry); err != nil {
			s.logger.Error("failed to create DLQ entry", "error", err)
		}

		s.publishEvent(n)
		s.logger.Warn("notification moved to DLQ",
			"notification_id", n.ID,
			"retry_count", n.RetryCount,
		)
		return
	}

	// Calculate next retry with exponential backoff + jitter
	delay := s.baseDelay * time.Duration(math.Pow(2, float64(n.RetryCount-1)))
	if delay > s.maxDelay {
		delay = s.maxDelay
	}
	// Add jitter: +/- 20%
	jitter := time.Duration(float64(delay) * (0.8 + rand.Float64()*0.4))
	nextRetry := time.Now().UTC().Add(jitter)

	n.Status = domain.StatusFailed
	n.NextRetryAt = &nextRetry
	_ = s.notifRepo.Update(ctx, n)

	s.publishEvent(n)
	s.logger.Info("notification scheduled for retry",
		"notification_id", n.ID,
		"retry_count", n.RetryCount,
		"next_retry_at", nextRetry,
	)
}

func (s *DeliveryService) publishEvent(n *domain.Notification) {
	evt := port.StatusEvent{
		NotificationID: n.ID.String(),
		Status:         string(n.Status),
		Channel:        string(n.Channel),
		Timestamp:      time.Now().UTC().Format(time.RFC3339),
	}
	if n.BatchID != nil {
		evt.BatchID = n.BatchID.String()
	}
	if n.ErrorMessage != nil {
		evt.Error = *n.ErrorMessage
	}
	s.event.Publish(evt)
}

func (s *DeliveryService) GetBreakers() map[domain.Channel]*circuitbreaker.CircuitBreaker {
	return s.breakers
}

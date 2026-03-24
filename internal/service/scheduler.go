package service

import (
	"context"
	"log/slog"
	"time"

	"github.com/softwarecase/event-driven-notification-service/internal/port"
)

type SchedulerService struct {
	repo     port.NotificationRepository
	queue    port.QueuePublisher
	logger   *slog.Logger
	interval time.Duration
}

func NewSchedulerService(
	repo port.NotificationRepository,
	queue port.QueuePublisher,
	logger *slog.Logger,
	interval time.Duration,
) *SchedulerService {
	return &SchedulerService{
		repo:     repo,
		queue:    queue,
		logger:   logger,
		interval: interval,
	}
}

// Run starts the scheduler loop that polls for ready scheduled notifications
func (s *SchedulerService) Run(ctx context.Context) {
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	s.logger.Info("scheduler started", "interval", s.interval)

	for {
		select {
		case <-ctx.Done():
			s.logger.Info("scheduler stopped")
			return
		case <-ticker.C:
			s.poll(ctx)
		}
	}
}

func (s *SchedulerService) poll(ctx context.Context) {
	notifications, err := s.repo.GetScheduledReady(ctx, 100)
	if err != nil {
		s.logger.Error("scheduler poll failed", "error", err)
		return
	}

	if len(notifications) == 0 {
		return
	}

	var msgs []port.QueueMessage
	for _, n := range notifications {
		msgs = append(msgs, port.QueueMessage{
			NotificationID: n.ID.String(),
			Channel:        string(n.Channel),
			Priority:       int(n.Priority),
		})
	}

	if err := s.queue.EnqueueBatch(ctx, msgs); err != nil {
		s.logger.Error("scheduler enqueue failed", "error", err, "count", len(msgs))
		return
	}

	s.logger.Info("scheduler enqueued notifications", "count", len(notifications))
}

// RetryPoller polls for notifications ready for retry
type RetryPoller struct {
	repo     port.NotificationRepository
	queue    port.QueuePublisher
	logger   *slog.Logger
	interval time.Duration
}

func NewRetryPoller(
	repo port.NotificationRepository,
	queue port.QueuePublisher,
	logger *slog.Logger,
	interval time.Duration,
) *RetryPoller {
	return &RetryPoller{
		repo:     repo,
		queue:    queue,
		logger:   logger,
		interval: interval,
	}
}

func (p *RetryPoller) Run(ctx context.Context) {
	ticker := time.NewTicker(p.interval)
	defer ticker.Stop()

	p.logger.Info("retry poller started", "interval", p.interval)

	for {
		select {
		case <-ctx.Done():
			p.logger.Info("retry poller stopped")
			return
		case <-ticker.C:
			p.poll(ctx)
		}
	}
}

func (p *RetryPoller) poll(ctx context.Context) {
	notifications, err := p.repo.GetRetryReady(ctx, 100)
	if err != nil {
		p.logger.Error("retry poll failed", "error", err)
		return
	}

	if len(notifications) == 0 {
		return
	}

	var msgs []port.QueueMessage
	for _, n := range notifications {
		msgs = append(msgs, port.QueueMessage{
			NotificationID: n.ID.String(),
			Channel:        string(n.Channel),
			Priority:       int(n.Priority),
		})
	}

	if err := p.queue.EnqueueBatch(ctx, msgs); err != nil {
		p.logger.Error("retry enqueue failed", "error", err, "count", len(msgs))
		return
	}

	p.logger.Info("retry poller enqueued notifications", "count", len(notifications))
}

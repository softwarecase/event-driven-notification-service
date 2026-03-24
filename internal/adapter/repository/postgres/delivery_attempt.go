package postgres

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/softwarecase/event-driven-notification-service/internal/domain"
)

type DeliveryAttemptRepo struct {
	pool *pgxpool.Pool
}

func NewDeliveryAttemptRepo(pool *pgxpool.Pool) *DeliveryAttemptRepo {
	return &DeliveryAttemptRepo{pool: pool}
}

func (r *DeliveryAttemptRepo) Create(ctx context.Context, a *domain.DeliveryAttempt) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO delivery_attempts (
			id, notification_id, attempt_number, status, provider_msg_id,
			status_code, response_body, error_message, duration_ms, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
		a.ID, a.NotificationID, a.AttemptNumber, a.Status, a.ProviderMsgID,
		a.StatusCode, a.ResponseBody, a.ErrorMessage, a.DurationMs, a.CreatedAt,
	)
	return err
}

func (r *DeliveryAttemptRepo) GetByNotificationID(ctx context.Context, notificationID uuid.UUID) ([]*domain.DeliveryAttempt, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, notification_id, attempt_number, status, provider_msg_id,
			status_code, response_body, error_message, duration_ms, created_at
		FROM delivery_attempts WHERE notification_id = $1
		ORDER BY attempt_number`, notificationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var attempts []*domain.DeliveryAttempt
	for rows.Next() {
		a := &domain.DeliveryAttempt{}
		if err := rows.Scan(
			&a.ID, &a.NotificationID, &a.AttemptNumber, &a.Status, &a.ProviderMsgID,
			&a.StatusCode, &a.ResponseBody, &a.ErrorMessage, &a.DurationMs, &a.CreatedAt,
		); err != nil {
			return nil, err
		}
		attempts = append(attempts, a)
	}
	return attempts, rows.Err()
}

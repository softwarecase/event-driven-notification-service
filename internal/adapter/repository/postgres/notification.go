package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/softwarecase/event-driven-notification-service/internal/domain"
	"github.com/softwarecase/event-driven-notification-service/internal/port"
)

type NotificationRepo struct {
	pool *pgxpool.Pool
}

func NewNotificationRepo(pool *pgxpool.Pool) *NotificationRepo {
	return &NotificationRepo{pool: pool}
}

func (r *NotificationRepo) Create(ctx context.Context, n *domain.Notification) error {
	metadata, _ := json.Marshal(n.Metadata)
	templateVars, _ := json.Marshal(n.TemplateVars)

	_, err := r.pool.Exec(ctx, `
		INSERT INTO notifications (
			id, batch_id, idempotency_key, channel, recipient, subject, content,
			priority, status, scheduled_at, metadata, template_id, template_vars,
			max_retries, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)`,
		n.ID, n.BatchID, n.IdempotencyKey, n.Channel, n.Recipient, n.Subject, n.Content,
		n.Priority, n.Status, n.ScheduledAt, metadata, n.TemplateID, templateVars,
		n.MaxRetries, n.CreatedAt, n.UpdatedAt,
	)
	return err
}

func (r *NotificationRepo) CreateBatch(ctx context.Context, notifications []*domain.Notification) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	batch := &pgx.Batch{}
	for _, n := range notifications {
		metadata, _ := json.Marshal(n.Metadata)
		templateVars, _ := json.Marshal(n.TemplateVars)
		batch.Queue(`
			INSERT INTO notifications (
				id, batch_id, idempotency_key, channel, recipient, subject, content,
				priority, status, scheduled_at, metadata, template_id, template_vars,
				max_retries, created_at, updated_at
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)`,
			n.ID, n.BatchID, n.IdempotencyKey, n.Channel, n.Recipient, n.Subject, n.Content,
			n.Priority, n.Status, n.ScheduledAt, metadata, n.TemplateID, templateVars,
			n.MaxRetries, n.CreatedAt, n.UpdatedAt,
		)
	}

	br := tx.SendBatch(ctx, batch)
	for range notifications {
		if _, err := br.Exec(); err != nil {
			_ = br.Close()
			return err
		}
	}
	_ = br.Close()

	return tx.Commit(ctx)
}

func (r *NotificationRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.Notification, error) {
	n := &domain.Notification{}
	var metadata, templateVars []byte

	err := r.pool.QueryRow(ctx, `
		SELECT id, batch_id, idempotency_key, channel, recipient, subject, content,
			priority, status, scheduled_at, sent_at, provider_msg_id, retry_count,
			max_retries, next_retry_at, metadata, template_id, template_vars,
			error_message, created_at, updated_at
		FROM notifications WHERE id = $1`, id,
	).Scan(
		&n.ID, &n.BatchID, &n.IdempotencyKey, &n.Channel, &n.Recipient, &n.Subject, &n.Content,
		&n.Priority, &n.Status, &n.ScheduledAt, &n.SentAt, &n.ProviderMsgID, &n.RetryCount,
		&n.MaxRetries, &n.NextRetryAt, &metadata, &n.TemplateID, &templateVars,
		&n.ErrorMessage, &n.CreatedAt, &n.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	_ = json.Unmarshal(metadata, &n.Metadata)
	_ = json.Unmarshal(templateVars, &n.TemplateVars)
	return n, nil
}

func (r *NotificationRepo) GetByBatchID(ctx context.Context, batchID uuid.UUID) ([]*domain.Notification, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, batch_id, idempotency_key, channel, recipient, subject, content,
			priority, status, scheduled_at, sent_at, provider_msg_id, retry_count,
			max_retries, next_retry_at, metadata, template_id, template_vars,
			error_message, created_at, updated_at
		FROM notifications WHERE batch_id = $1
		ORDER BY created_at`, batchID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanNotifications(rows)
}

func (r *NotificationRepo) Update(ctx context.Context, n *domain.Notification) error {
	metadata, _ := json.Marshal(n.Metadata)
	n.UpdatedAt = time.Now().UTC()

	_, err := r.pool.Exec(ctx, `
		UPDATE notifications SET
			status = $2, sent_at = $3, provider_msg_id = $4, retry_count = $5,
			next_retry_at = $6, metadata = $7, error_message = $8, updated_at = $9
		WHERE id = $1`,
		n.ID, n.Status, n.SentAt, n.ProviderMsgID, n.RetryCount,
		n.NextRetryAt, metadata, n.ErrorMessage, n.UpdatedAt,
	)
	return err
}

func (r *NotificationRepo) UpdateStatus(ctx context.Context, id uuid.UUID, status domain.Status) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE notifications SET status = $2, updated_at = NOW() WHERE id = $1`,
		id, status,
	)
	return err
}

func (r *NotificationRepo) CancelByID(ctx context.Context, id uuid.UUID) error {
	result, err := r.pool.Exec(ctx, `
		UPDATE notifications SET status = 'cancelled', updated_at = NOW()
		WHERE id = $1 AND status IN ('pending', 'scheduled', 'queued')`, id)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		// Check if notification exists
		var exists bool
		_ = r.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM notifications WHERE id = $1)`, id).Scan(&exists)
		if !exists {
			return domain.ErrNotFound
		}
		return domain.ErrCannotCancel
	}
	return nil
}

func (r *NotificationRepo) CancelByBatchID(ctx context.Context, batchID uuid.UUID) (int64, error) {
	result, err := r.pool.Exec(ctx, `
		UPDATE notifications SET status = 'cancelled', updated_at = NOW()
		WHERE batch_id = $1 AND status IN ('pending', 'scheduled', 'queued')`, batchID)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected(), nil
}

func (r *NotificationRepo) List(ctx context.Context, filter port.NotificationFilter) (*port.PaginatedResult, error) {
	var conditions []string
	var args []interface{}
	argIdx := 1

	if filter.Status != nil {
		conditions = append(conditions, fmt.Sprintf("status = $%d", argIdx))
		args = append(args, *filter.Status)
		argIdx++
	}
	if filter.Channel != nil {
		conditions = append(conditions, fmt.Sprintf("channel = $%d", argIdx))
		args = append(args, *filter.Channel)
		argIdx++
	}
	if filter.BatchID != nil {
		conditions = append(conditions, fmt.Sprintf("batch_id = $%d", argIdx))
		args = append(args, *filter.BatchID)
		argIdx++
	}
	if filter.FromDate != nil {
		conditions = append(conditions, fmt.Sprintf("created_at >= $%d", argIdx))
		args = append(args, *filter.FromDate)
		argIdx++
	}
	if filter.ToDate != nil {
		conditions = append(conditions, fmt.Sprintf("created_at <= $%d", argIdx))
		args = append(args, *filter.ToDate)
		argIdx++
	}

	where := ""
	if len(conditions) > 0 {
		where = "WHERE " + strings.Join(conditions, " AND ")
	}

	// Count total
	var total int64
	countQuery := "SELECT COUNT(*) FROM notifications " + where
	if err := r.pool.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, err
	}

	// Pagination defaults
	if filter.Page < 1 {
		filter.Page = 1
	}
	if filter.PageSize < 1 || filter.PageSize > 100 {
		filter.PageSize = 20
	}
	offset := (filter.Page - 1) * filter.PageSize

	query := fmt.Sprintf(`
		SELECT id, batch_id, idempotency_key, channel, recipient, subject, content,
			priority, status, scheduled_at, sent_at, provider_msg_id, retry_count,
			max_retries, next_retry_at, metadata, template_id, template_vars,
			error_message, created_at, updated_at
		FROM notifications %s
		ORDER BY created_at DESC
		LIMIT $%d OFFSET $%d`, where, argIdx, argIdx+1)
	args = append(args, filter.PageSize, offset)

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	data, err := scanNotifications(rows)
	if err != nil {
		return nil, err
	}

	totalPages := int(total) / filter.PageSize
	if int(total)%filter.PageSize > 0 {
		totalPages++
	}

	return &port.PaginatedResult{
		Data:       data,
		Page:       filter.Page,
		PageSize:   filter.PageSize,
		Total:      total,
		TotalPages: totalPages,
	}, nil
}

func (r *NotificationRepo) GetScheduledReady(ctx context.Context, limit int) ([]*domain.Notification, error) {
	rows, err := r.pool.Query(ctx, `
		UPDATE notifications
		SET status = 'queued', updated_at = NOW()
		WHERE id IN (
			SELECT id FROM notifications
			WHERE status = 'scheduled' AND scheduled_at <= NOW()
			ORDER BY scheduled_at
			LIMIT $1
			FOR UPDATE SKIP LOCKED
		)
		RETURNING id, batch_id, idempotency_key, channel, recipient, subject, content,
			priority, status, scheduled_at, sent_at, provider_msg_id, retry_count,
			max_retries, next_retry_at, metadata, template_id, template_vars,
			error_message, created_at, updated_at`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanNotifications(rows)
}

func (r *NotificationRepo) GetRetryReady(ctx context.Context, limit int) ([]*domain.Notification, error) {
	rows, err := r.pool.Query(ctx, `
		UPDATE notifications
		SET status = 'queued', updated_at = NOW()
		WHERE id IN (
			SELECT id FROM notifications
			WHERE status = 'failed'
				AND retry_count < max_retries
				AND next_retry_at <= NOW()
			ORDER BY next_retry_at
			LIMIT $1
			FOR UPDATE SKIP LOCKED
		)
		RETURNING id, batch_id, idempotency_key, channel, recipient, subject, content,
			priority, status, scheduled_at, sent_at, provider_msg_id, retry_count,
			max_retries, next_retry_at, metadata, template_id, template_vars,
			error_message, created_at, updated_at`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanNotifications(rows)
}

func (r *NotificationRepo) GetByIdempotencyKey(ctx context.Context, key string) (*domain.Notification, error) {
	n := &domain.Notification{}
	var metadata, templateVars []byte

	err := r.pool.QueryRow(ctx, `
		SELECT id, batch_id, idempotency_key, channel, recipient, subject, content,
			priority, status, scheduled_at, sent_at, provider_msg_id, retry_count,
			max_retries, next_retry_at, metadata, template_id, template_vars,
			error_message, created_at, updated_at
		FROM notifications WHERE idempotency_key = $1`, key,
	).Scan(
		&n.ID, &n.BatchID, &n.IdempotencyKey, &n.Channel, &n.Recipient, &n.Subject, &n.Content,
		&n.Priority, &n.Status, &n.ScheduledAt, &n.SentAt, &n.ProviderMsgID, &n.RetryCount,
		&n.MaxRetries, &n.NextRetryAt, &metadata, &n.TemplateID, &templateVars,
		&n.ErrorMessage, &n.CreatedAt, &n.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	_ = json.Unmarshal(metadata, &n.Metadata)
	_ = json.Unmarshal(templateVars, &n.TemplateVars)
	return n, nil
}

func scanNotifications(rows pgx.Rows) ([]*domain.Notification, error) {
	var result []*domain.Notification
	for rows.Next() {
		n := &domain.Notification{}
		var metadata, templateVars []byte
		err := rows.Scan(
			&n.ID, &n.BatchID, &n.IdempotencyKey, &n.Channel, &n.Recipient, &n.Subject, &n.Content,
			&n.Priority, &n.Status, &n.ScheduledAt, &n.SentAt, &n.ProviderMsgID, &n.RetryCount,
			&n.MaxRetries, &n.NextRetryAt, &metadata, &n.TemplateID, &templateVars,
			&n.ErrorMessage, &n.CreatedAt, &n.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		_ = json.Unmarshal(metadata, &n.Metadata)
		_ = json.Unmarshal(templateVars, &n.TemplateVars)
		result = append(result, n)
	}
	return result, rows.Err()
}

package postgres

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/softwarecase/event-driven-notification-service/internal/domain"
)

type DeadLetterRepo struct {
	pool *pgxpool.Pool
}

func NewDeadLetterRepo(pool *pgxpool.Pool) *DeadLetterRepo {
	return &DeadLetterRepo{pool: pool}
}

func (r *DeadLetterRepo) Create(ctx context.Context, entry *domain.DeadLetterEntry) error {
	payload, _ := json.Marshal(entry.Payload)
	_, err := r.pool.Exec(ctx, `
		INSERT INTO dead_letter_queue (id, notification_id, reason, last_error, payload, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)`,
		entry.ID, entry.NotificationID, entry.Reason, entry.LastError, payload, entry.CreatedAt,
	)
	return err
}

func (r *DeadLetterRepo) List(ctx context.Context, page, pageSize int) ([]*domain.DeadLetterEntry, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize

	rows, err := r.pool.Query(ctx, `
		SELECT id, notification_id, reason, last_error, payload, created_at, reprocessed_at
		FROM dead_letter_queue ORDER BY created_at DESC
		LIMIT $1 OFFSET $2`, pageSize, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []*domain.DeadLetterEntry
	for rows.Next() {
		e := &domain.DeadLetterEntry{}
		var payload []byte
		if err := rows.Scan(&e.ID, &e.NotificationID, &e.Reason, &e.LastError, &payload, &e.CreatedAt, &e.ReprocessedAt); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(payload, &e.Payload)
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

func (r *DeadLetterRepo) MarkReprocessed(ctx context.Context, id uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE dead_letter_queue SET reprocessed_at = $2 WHERE id = $1`,
		id, time.Now().UTC())
	return err
}

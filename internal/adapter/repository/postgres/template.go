package postgres

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/softwarecase/event-driven-notification-service/internal/domain"
)

type TemplateRepo struct {
	pool *pgxpool.Pool
}

func NewTemplateRepo(pool *pgxpool.Pool) *TemplateRepo {
	return &TemplateRepo{pool: pool}
}

func (r *TemplateRepo) Create(ctx context.Context, t *domain.Template) error {
	variables, _ := json.Marshal(t.Variables)
	_, err := r.pool.Exec(ctx, `
		INSERT INTO templates (id, name, channel, subject, content, variables, active, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		t.ID, t.Name, t.Channel, t.Subject, t.Content, variables, t.Active, t.CreatedAt, t.UpdatedAt,
	)
	return err
}

func (r *TemplateRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.Template, error) {
	t := &domain.Template{}
	var variables []byte
	err := r.pool.QueryRow(ctx, `
		SELECT id, name, channel, subject, content, variables, active, created_at, updated_at
		FROM templates WHERE id = $1`, id,
	).Scan(&t.ID, &t.Name, &t.Channel, &t.Subject, &t.Content, &variables, &t.Active, &t.CreatedAt, &t.UpdatedAt)
	if err == pgx.ErrNoRows {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	_ = json.Unmarshal(variables, &t.Variables)
	return t, nil
}

func (r *TemplateRepo) GetByName(ctx context.Context, name string) (*domain.Template, error) {
	t := &domain.Template{}
	var variables []byte
	err := r.pool.QueryRow(ctx, `
		SELECT id, name, channel, subject, content, variables, active, created_at, updated_at
		FROM templates WHERE name = $1`, name,
	).Scan(&t.ID, &t.Name, &t.Channel, &t.Subject, &t.Content, &variables, &t.Active, &t.CreatedAt, &t.UpdatedAt)
	if err == pgx.ErrNoRows {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	_ = json.Unmarshal(variables, &t.Variables)
	return t, nil
}

func (r *TemplateRepo) Update(ctx context.Context, t *domain.Template) error {
	variables, _ := json.Marshal(t.Variables)
	t.UpdatedAt = time.Now().UTC()
	_, err := r.pool.Exec(ctx, `
		UPDATE templates SET name=$2, channel=$3, subject=$4, content=$5, variables=$6, active=$7, updated_at=$8
		WHERE id = $1`,
		t.ID, t.Name, t.Channel, t.Subject, t.Content, variables, t.Active, t.UpdatedAt,
	)
	return err
}

func (r *TemplateRepo) Delete(ctx context.Context, id uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `UPDATE templates SET active = false, updated_at = NOW() WHERE id = $1`, id)
	return err
}

func (r *TemplateRepo) List(ctx context.Context, page, pageSize int) ([]*domain.Template, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	var total int64
	if err := r.pool.QueryRow(ctx, `SELECT COUNT(*) FROM templates WHERE active = true`).Scan(&total); err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	rows, err := r.pool.Query(ctx, `
		SELECT id, name, channel, subject, content, variables, active, created_at, updated_at
		FROM templates WHERE active = true
		ORDER BY created_at DESC
		LIMIT $1 OFFSET $2`, pageSize, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var templates []*domain.Template
	for rows.Next() {
		t := &domain.Template{}
		var variables []byte
		if err := rows.Scan(&t.ID, &t.Name, &t.Channel, &t.Subject, &t.Content, &variables, &t.Active, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, 0, err
		}
		_ = json.Unmarshal(variables, &t.Variables)
		templates = append(templates, t)
	}

	return templates, total, rows.Err()
}

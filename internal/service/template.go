package service

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"text/template"

	"github.com/google/uuid"
	"github.com/softwarecase/event-driven-notification-service/internal/domain"
	"github.com/softwarecase/event-driven-notification-service/internal/port"
)

type TemplateService struct {
	repo   port.TemplateRepository
	logger *slog.Logger
}

func NewTemplateService(repo port.TemplateRepository, logger *slog.Logger) *TemplateService {
	return &TemplateService{repo: repo, logger: logger}
}

type CreateTemplateRequest struct {
	Name      string                    `json:"name" validate:"required"`
	Channel   string                    `json:"channel" validate:"required,oneof=sms email push"`
	Subject   string                    `json:"subject,omitempty"`
	Content   string                    `json:"content" validate:"required"`
	Variables []domain.TemplateVariable `json:"variables,omitempty"`
}

type UpdateTemplateRequest struct {
	Name      *string                    `json:"name,omitempty"`
	Channel   *string                    `json:"channel,omitempty" validate:"omitempty,oneof=sms email push"`
	Subject   *string                    `json:"subject,omitempty"`
	Content   *string                    `json:"content,omitempty"`
	Variables *[]domain.TemplateVariable `json:"variables,omitempty"`
}

func (s *TemplateService) Create(ctx context.Context, req CreateTemplateRequest) (*domain.Template, error) {
	channel := domain.Channel(req.Channel)
	if !channel.IsValid() {
		return nil, domain.ErrInvalidChannel
	}

	// Validate template syntax
	if _, err := template.New("test").Parse(req.Content); err != nil {
		return nil, fmt.Errorf("invalid template syntax: %w", err)
	}

	t := domain.NewTemplate(req.Name, channel, req.Content)
	t.Subject = req.Subject
	if req.Variables != nil {
		t.Variables = req.Variables
	}

	if err := s.repo.Create(ctx, t); err != nil {
		return nil, err
	}

	return t, nil
}

func (s *TemplateService) GetByID(ctx context.Context, id uuid.UUID) (*domain.Template, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *TemplateService) Update(ctx context.Context, id uuid.UUID, req UpdateTemplateRequest) (*domain.Template, error) {
	t, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if req.Name != nil {
		t.Name = *req.Name
	}
	if req.Channel != nil {
		channel := domain.Channel(*req.Channel)
		if !channel.IsValid() {
			return nil, domain.ErrInvalidChannel
		}
		t.Channel = channel
	}
	if req.Subject != nil {
		t.Subject = *req.Subject
	}
	if req.Content != nil {
		if _, err := template.New("test").Parse(*req.Content); err != nil {
			return nil, fmt.Errorf("invalid template syntax: %w", err)
		}
		t.Content = *req.Content
	}
	if req.Variables != nil {
		t.Variables = *req.Variables
	}

	if err := s.repo.Update(ctx, t); err != nil {
		return nil, err
	}

	return t, nil
}

func (s *TemplateService) Delete(ctx context.Context, id uuid.UUID) error {
	return s.repo.Delete(ctx, id)
}

func (s *TemplateService) List(ctx context.Context, page, pageSize int) ([]*domain.Template, int64, error) {
	return s.repo.List(ctx, page, pageSize)
}

// Render renders a template with the given variables
func (s *TemplateService) Render(ctx context.Context, tmpl *domain.Template, vars map[string]interface{}) (subject, content string, err error) {
	// Validate required variables
	if vars == nil {
		vars = make(map[string]interface{})
	}

	for _, v := range tmpl.Variables {
		if _, ok := vars[v.Name]; !ok {
			if v.Required {
				return "", "", fmt.Errorf("%w: %s", domain.ErrMissingTemplateVars, v.Name)
			}
			if v.Default != nil {
				vars[v.Name] = *v.Default
			}
		}
	}

	// Render content
	contentTmpl, err := template.New("content").Parse(tmpl.Content)
	if err != nil {
		return "", "", fmt.Errorf("parse content template: %w", err)
	}
	var contentBuf bytes.Buffer
	if err := contentTmpl.Execute(&contentBuf, vars); err != nil {
		return "", "", fmt.Errorf("execute content template: %w", err)
	}

	// Render subject if present
	if tmpl.Subject != "" {
		subjectTmpl, err := template.New("subject").Parse(tmpl.Subject)
		if err != nil {
			return "", "", fmt.Errorf("parse subject template: %w", err)
		}
		var subjectBuf bytes.Buffer
		if err := subjectTmpl.Execute(&subjectBuf, vars); err != nil {
			return "", "", fmt.Errorf("execute subject template: %w", err)
		}
		subject = subjectBuf.String()
	}

	return subject, contentBuf.String(), nil
}

// Preview renders a template without saving
func (s *TemplateService) Preview(ctx context.Context, id uuid.UUID, vars map[string]interface{}) (subject, content string, err error) {
	tmpl, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return "", "", err
	}
	if !tmpl.Active {
		return "", "", domain.ErrTemplateInactive
	}
	return s.Render(ctx, tmpl, vars)
}

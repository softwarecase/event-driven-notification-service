package service

import (
	"context"
	"log/slog"
	"os"
	"testing"

	"github.com/softwarecase/event-driven-notification-service/internal/domain"
	"github.com/stretchr/testify/assert"
)

func TestTemplateService_Render(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	svc := NewTemplateService(nil, logger)

	tmpl := &domain.Template{
		Content: "Hello {{.Name}}, your order {{.OrderID}} is ready!",
		Subject: "Order {{.OrderID}} Update",
		Variables: []domain.TemplateVariable{
			{Name: "Name", Required: true},
			{Name: "OrderID", Required: true},
		},
	}

	vars := map[string]interface{}{
		"Name":    "Ahmet",
		"OrderID": "12345",
	}

	subject, content, err := svc.Render(context.Background(), tmpl, vars)

	assert.NoError(t, err)
	assert.Equal(t, "Order 12345 Update", subject)
	assert.Equal(t, "Hello Ahmet, your order 12345 is ready!", content)
}

func TestTemplateService_Render_MissingRequired(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	svc := NewTemplateService(nil, logger)

	tmpl := &domain.Template{
		Content: "Hello {{.Name}}",
		Variables: []domain.TemplateVariable{
			{Name: "Name", Required: true},
		},
	}

	_, _, err := svc.Render(context.Background(), tmpl, map[string]interface{}{})
	assert.ErrorIs(t, err, domain.ErrMissingTemplateVars)
}

func TestTemplateService_Render_DefaultValues(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	svc := NewTemplateService(nil, logger)

	defaultEmail := "support@example.com"
	tmpl := &domain.Template{
		Content: "Contact us at {{.Email}}",
		Variables: []domain.TemplateVariable{
			{Name: "Email", Required: false, Default: &defaultEmail},
		},
	}

	_, content, err := svc.Render(context.Background(), tmpl, map[string]interface{}{})
	assert.NoError(t, err)
	assert.Equal(t, "Contact us at support@example.com", content)
}

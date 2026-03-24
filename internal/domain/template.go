package domain

import (
	"time"

	"github.com/google/uuid"
)

type TemplateVariable struct {
	Name     string  `json:"name"`
	Required bool    `json:"required"`
	Default  *string `json:"default,omitempty"`
}

type Template struct {
	ID        uuid.UUID          `json:"id"`
	Name      string             `json:"name"`
	Channel   Channel            `json:"channel"`
	Subject   string             `json:"subject,omitempty"`
	Content   string             `json:"content"`
	Variables []TemplateVariable `json:"variables"`
	Active    bool               `json:"active"`
	CreatedAt time.Time          `json:"created_at"`
	UpdatedAt time.Time          `json:"updated_at"`
}

func NewTemplate(name string, channel Channel, content string) *Template {
	now := time.Now().UTC()
	return &Template{
		ID:        uuid.New(),
		Name:      name,
		Channel:   channel,
		Content:   content,
		Variables: make([]TemplateVariable, 0),
		Active:    true,
		CreatedAt: now,
		UpdatedAt: now,
	}
}

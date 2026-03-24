package service

import (
	"testing"

	"github.com/softwarecase/event-driven-notification-service/internal/domain"
	"github.com/stretchr/testify/assert"
)

func TestValidateContent(t *testing.T) {
	tests := []struct {
		name    string
		channel domain.Channel
		content string
		wantErr error
	}{
		{"valid sms", domain.ChannelSMS, "Hello", nil},
		{"empty content", domain.ChannelSMS, "", domain.ErrEmptyContent},
		{"sms too long", domain.ChannelSMS, string(make([]byte, 1601)), domain.ErrContentTooLong},
		{"push too long", domain.ChannelPush, string(make([]byte, 4097)), domain.ErrContentTooLong},
		{"email too long", domain.ChannelEmail, string(make([]byte, 100001)), domain.ErrContentTooLong},
		{"valid email", domain.ChannelEmail, "Hello World", nil},
		{"valid push", domain.ChannelPush, "Notification", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateContent(tt.channel, tt.content)
			if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

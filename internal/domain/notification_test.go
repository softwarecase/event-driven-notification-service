package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestChannel_IsValid(t *testing.T) {
	tests := []struct {
		channel Channel
		valid   bool
	}{
		{ChannelSMS, true},
		{ChannelEmail, true},
		{ChannelPush, true},
		{Channel("whatsapp"), false},
		{Channel(""), false},
	}

	for _, tt := range tests {
		t.Run(string(tt.channel), func(t *testing.T) {
			assert.Equal(t, tt.valid, tt.channel.IsValid())
		})
	}
}

func TestPriority_FromString(t *testing.T) {
	tests := []struct {
		input    string
		expected Priority
	}{
		{"high", PriorityHigh},
		{"normal", PriorityNormal},
		{"low", PriorityLow},
		{"", PriorityNormal},
		{"invalid", PriorityNormal},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			assert.Equal(t, tt.expected, PriorityFromString(tt.input))
		})
	}
}

func TestPriority_String(t *testing.T) {
	assert.Equal(t, "high", PriorityHigh.String())
	assert.Equal(t, "normal", PriorityNormal.String())
	assert.Equal(t, "low", PriorityLow.String())
}

func TestStatus_IsFinal(t *testing.T) {
	assert.True(t, StatusDelivered.IsFinal())
	assert.True(t, StatusFailed.IsFinal())
	assert.True(t, StatusCancelled.IsFinal())
	assert.False(t, StatusPending.IsFinal())
	assert.False(t, StatusQueued.IsFinal())
	assert.False(t, StatusProcessing.IsFinal())
	assert.False(t, StatusScheduled.IsFinal())
}

func TestStatus_CanCancel(t *testing.T) {
	assert.True(t, StatusPending.CanCancel())
	assert.True(t, StatusScheduled.CanCancel())
	assert.True(t, StatusQueued.CanCancel())
	assert.False(t, StatusProcessing.CanCancel())
	assert.False(t, StatusDelivered.CanCancel())
	assert.False(t, StatusFailed.CanCancel())
}

func TestNewNotification(t *testing.T) {
	n := NewNotification(ChannelSMS, "+905551234567", "Hello World", PriorityHigh)

	assert.NotEmpty(t, n.ID)
	assert.Equal(t, ChannelSMS, n.Channel)
	assert.Equal(t, "+905551234567", n.Recipient)
	assert.Equal(t, "Hello World", n.Content)
	assert.Equal(t, PriorityHigh, n.Priority)
	assert.Equal(t, StatusPending, n.Status)
	assert.Equal(t, 0, n.RetryCount)
	assert.Equal(t, 3, n.MaxRetries)
	assert.NotNil(t, n.Metadata)
	assert.NotZero(t, n.CreatedAt)
	assert.NotZero(t, n.UpdatedAt)
}

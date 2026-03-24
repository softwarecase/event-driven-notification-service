package domain

import "errors"

var (
	ErrNotFound            = errors.New("resource not found")
	ErrDuplicateIDKey      = errors.New("duplicate idempotency key")
	ErrInvalidChannel      = errors.New("invalid channel: must be sms, email, or push")
	ErrInvalidPriority     = errors.New("invalid priority: must be high, normal, or low")
	ErrInvalidStatus       = errors.New("invalid notification status")
	ErrCannotCancel        = errors.New("notification cannot be cancelled in current status")
	ErrBatchTooLarge       = errors.New("batch size exceeds maximum of 1000")
	ErrContentTooLong      = errors.New("content exceeds maximum length for channel")
	ErrEmptyRecipient      = errors.New("recipient is required")
	ErrEmptyContent        = errors.New("content is required")
	ErrTemplateNotFound    = errors.New("template not found")
	ErrTemplateInactive    = errors.New("template is not active")
	ErrMissingTemplateVars = errors.New("missing required template variables")
	ErrCircuitOpen         = errors.New("circuit breaker is open for this channel")
	ErrRateLimited         = errors.New("rate limit exceeded for channel")
	ErrScheduleInPast      = errors.New("scheduled_at must be in the future")
)

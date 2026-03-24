package domain

import "github.com/google/uuid"

const MaxBatchSize = 1000

type BatchResult struct {
	BatchID       uuid.UUID    `json:"batch_id"`
	Total         int          `json:"total"`
	Accepted      int          `json:"accepted"`
	Rejected      int          `json:"rejected"`
	Notifications []uuid.UUID  `json:"notification_ids"`
	Errors        []BatchError `json:"errors,omitempty"`
}

type BatchError struct {
	Index   int    `json:"index"`
	Message string `json:"message"`
}

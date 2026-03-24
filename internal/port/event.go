package port

type StatusEvent struct {
	NotificationID string `json:"notification_id"`
	BatchID        string `json:"batch_id,omitempty"`
	Status         string `json:"status"`
	Channel        string `json:"channel"`
	Timestamp      string `json:"timestamp"`
	Error          string `json:"error,omitempty"`
}

type EventPublisher interface {
	Publish(event StatusEvent)
}

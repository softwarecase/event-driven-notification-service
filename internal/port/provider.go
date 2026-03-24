package port

import "context"

type SendRequest struct {
	To      string `json:"to"`
	Channel string `json:"channel"`
	Content string `json:"content"`
}

type SendResponse struct {
	MessageID string `json:"messageId"`
	Status    string `json:"status"`
	Timestamp string `json:"timestamp"`
}

type DeliveryProvider interface {
	Send(ctx context.Context, req SendRequest) (*SendResponse, error)
}

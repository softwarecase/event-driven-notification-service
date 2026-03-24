package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/softwarecase/event-driven-notification-service/internal/port"
)

type WebhookProvider struct {
	url    string
	client *http.Client
}

func NewWebhookProvider(webhookURL string, timeout time.Duration) *WebhookProvider {
	return &WebhookProvider{
		url: webhookURL,
		client: &http.Client{
			Timeout: timeout,
		},
	}
}

func (p *WebhookProvider) Send(ctx context.Context, req port.SendRequest) (*port.SendResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("provider returned status %d: %s", resp.StatusCode, string(respBody))
	}

	var sendResp port.SendResponse
	if err := json.Unmarshal(respBody, &sendResp); err != nil {
		sendResp = port.SendResponse{
			MessageID: "",
			Status:    "accepted",
			Timestamp: time.Now().UTC().Format(time.RFC3339),
		}
	}

	return &sendResp, nil
}

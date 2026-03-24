package handler

import (
	"context"
	"net/http"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/softwarecase/event-driven-notification-service/internal/domain"
	"github.com/softwarecase/event-driven-notification-service/internal/port"
	"github.com/softwarecase/event-driven-notification-service/pkg/circuitbreaker"
)

const (
	metricsKeyDelivered  = "metrics:delivered"
	metricsKeyFailed     = "metrics:failed"
	metricsKeyLatencySum = "metrics:latency_sum"
	metricsKeyProcessed  = "metrics:processed"
)

// MetricsCollector records delivery metrics in Redis 
type MetricsCollector struct {
	client *redis.Client
}

func NewMetricsCollector(client *redis.Client) *MetricsCollector {
	return &MetricsCollector{client: client}
}

func (m *MetricsCollector) RecordDelivery(latencyMs int64) {
	ctx := context.Background()
	pipe := m.client.Pipeline()
	pipe.Incr(ctx, metricsKeyDelivered)
	pipe.IncrBy(ctx, metricsKeyLatencySum, latencyMs)
	pipe.Incr(ctx, metricsKeyProcessed)
	_, _ = pipe.Exec(ctx)
}

func (m *MetricsCollector) RecordFailure() {
	ctx := context.Background()
	pipe := m.client.Pipeline()
	pipe.Incr(ctx, metricsKeyFailed)
	pipe.Incr(ctx, metricsKeyProcessed)
	_, _ = pipe.Exec(ctx)
}

type MetricsHandler struct {
	redis    *redis.Client
	consumer port.QueueConsumer
	breakers map[domain.Channel]*circuitbreaker.CircuitBreaker
}

func NewMetricsHandler(
	redisClient *redis.Client,
	consumer port.QueueConsumer,
	breakers map[domain.Channel]*circuitbreaker.CircuitBreaker,
) *MetricsHandler {
	return &MetricsHandler{
		redis:    redisClient,
		consumer: consumer,
		breakers: breakers,
	}
}

func (h *MetricsHandler) Get(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	channels := []domain.Channel{domain.ChannelSMS, domain.ChannelEmail, domain.ChannelPush}
	queueDepths := make(map[string]int64)
	circuitStates := make(map[string]string)

	for _, ch := range channels {
		depth, err := h.consumer.QueueDepth(ctx, ch)
		if err == nil {
			queueDepths[string(ch)] = depth
		}
		if breaker, ok := h.breakers[ch]; ok {
			circuitStates[string(ch)] = breaker.State().String()
		}
	}

	delivered, _ := h.redis.Get(ctx, metricsKeyDelivered).Int64()
	failed, _ := h.redis.Get(ctx, metricsKeyFailed).Int64()
	latencySum, _ := h.redis.Get(ctx, metricsKeyLatencySum).Int64()
	totalProcessed, _ := h.redis.Get(ctx, metricsKeyProcessed).Int64()

	var avgLatency float64
	if totalProcessed > 0 {
		avgLatency = float64(latencySum) / float64(totalProcessed)
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"queue_depth":      queueDepths,
		"delivered":        delivered,
		"failed":           failed,
		"total_processed":  totalProcessed,
		"avg_latency_ms":   avgLatency,
		"circuit_breakers": circuitStates,
		"timestamp":        time.Now().UTC().Format(time.RFC3339),
	})
}

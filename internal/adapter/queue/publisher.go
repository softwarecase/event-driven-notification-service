package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/softwarecase/event-driven-notification-service/internal/port"
)

const queueKeyPrefix = "queue:notifications:"

type RedisPublisher struct {
	client *redis.Client
}

func NewRedisPublisher(client *redis.Client) *RedisPublisher {
	return &RedisPublisher{client: client}
}

func (p *RedisPublisher) Enqueue(ctx context.Context, msg port.QueueMessage) error {
	key := queueKeyPrefix + msg.Channel
	score := calcScore(msg.Priority)

	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("marshal queue message: %w", err)
	}

	return p.client.ZAdd(ctx, key, redis.Z{
		Score:  score,
		Member: string(data),
	}).Err()
}

func (p *RedisPublisher) EnqueueBatch(ctx context.Context, msgs []port.QueueMessage) error {
	pipe := p.client.Pipeline()
	for _, msg := range msgs {
		key := queueKeyPrefix + msg.Channel
		score := calcScore(msg.Priority)
		data, err := json.Marshal(msg)
		if err != nil {
			return fmt.Errorf("marshal queue message: %w", err)
		}
		pipe.ZAdd(ctx, key, redis.Z{
			Score:  score,
			Member: string(data),
		})
	}
	_, err := pipe.Exec(ctx)
	return err
}

// calcScore generates a score for Redis sorted set:
// priority_weight * 1e13 + unix_timestamp_nanos / 1e6
// Lower score = higher priority + earlier timestamp
func calcScore(priority int) float64 {
	return float64(priority)*1e13 + float64(time.Now().UnixNano()/1e6)
}

package queue

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/redis/go-redis/v9"
	"github.com/softwarecase/event-driven-notification-service/internal/domain"
	"github.com/softwarecase/event-driven-notification-service/internal/port"
)

const processingKeyPrefix = "processing:notifications:"

// Lua script to atomically pop the lowest-scored member from sorted set
// and add to processing set
var dequeueScript = redis.NewScript(`
local member = redis.call('ZRANGEBYSCORE', KEYS[1], '-inf', '+inf', 'LIMIT', 0, 1)
if #member > 0 then
    redis.call('ZREM', KEYS[1], member[1])
    redis.call('SADD', KEYS[2], member[1])
    return member[1]
end
return nil
`)

type RedisConsumer struct {
	client *redis.Client
}

func NewRedisConsumer(client *redis.Client) *RedisConsumer {
	return &RedisConsumer{client: client}
}

func (c *RedisConsumer) Dequeue(ctx context.Context, channel domain.Channel) (*port.QueueMessage, error) {
	queueKey := queueKeyPrefix + string(channel)
	processingKey := processingKeyPrefix + string(channel)

	result, err := dequeueScript.Run(ctx, c.client, []string{queueKey, processingKey}).Result()
	if err == redis.Nil || result == nil {
		return nil, nil // empty queue
	}
	if err != nil {
		return nil, fmt.Errorf("dequeue script: %w", err)
	}

	var msg port.QueueMessage
	if err := json.Unmarshal([]byte(result.(string)), &msg); err != nil {
		return nil, fmt.Errorf("unmarshal queue message: %w", err)
	}

	return &msg, nil
}

func (c *RedisConsumer) Acknowledge(ctx context.Context, channel domain.Channel, notificationID string) error {
	processingKey := processingKeyPrefix + string(channel)
	members, err := c.client.SMembers(ctx, processingKey).Result()
	if err != nil {
		return err
	}
	for _, member := range members {
		var msg port.QueueMessage
		if err := json.Unmarshal([]byte(member), &msg); err != nil {
			continue
		}
		if msg.NotificationID == notificationID {
			return c.client.SRem(ctx, processingKey, member).Err()
		}
	}
	return nil
}

func (c *RedisConsumer) QueueDepth(ctx context.Context, channel domain.Channel) (int64, error) {
	return c.client.ZCard(ctx, queueKeyPrefix+string(channel)).Result()
}

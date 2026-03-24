package ratelimit

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// Sliding window rate limiter using Redis sorted sets
var slidingWindowScript = redis.NewScript(`
local key = KEYS[1]
local limit = tonumber(ARGV[1])
local window = tonumber(ARGV[2])
local now = tonumber(ARGV[3])
local unique_id = ARGV[4]

redis.call('ZREMRANGEBYSCORE', key, 0, now - window)

local count = redis.call('ZCARD', key)

if count < limit then
    redis.call('ZADD', key, now, unique_id)
    redis.call('PEXPIRE', key, window)
    return 1
end
return 0
`)

type Limiter struct {
	client      *redis.Client
	limitPerSec int
	windowMs    int
}

func NewLimiter(client *redis.Client, limitPerSec int) *Limiter {
	return &Limiter{
		client:      client,
		limitPerSec: limitPerSec,
		windowMs:    1000, // 1 second window
	}
}

// Allow checks if the request is within rate limits for the given channel
func (l *Limiter) Allow(ctx context.Context, channel string) (bool, error) {
	key := fmt.Sprintf("ratelimit:%s", channel)
	now := time.Now().UnixMilli()
	uniqueID := uuid.New().String()

	result, err := slidingWindowScript.Run(ctx, l.client, []string{key}, l.limitPerSec, l.windowMs, now, uniqueID).Int()
	if err != nil {
		return false, fmt.Errorf("rate limit script: %w", err)
	}

	return result == 1, nil
}

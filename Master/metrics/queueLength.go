package metrics

import (
	"KubeLite/data"
	"context"
	"fmt"

	"github.com/redis/go-redis/v9"
)

// GetQueueLength returns the number of pending tasks in the Redis queue ("task_queue").
func GetQueueLength() (int64, error) {
	ctx := context.Background()
	if data.RedisClient == nil {
		return 0, fmt.Errorf("redis client is not initialized")
	}

	length, err := data.RedisClient.LLen(ctx, "task_queue").Result()
	if err == redis.Nil {
		return 0, nil
	} else if err != nil {
		return 0, fmt.Errorf("failed to get queue length from redis: %w", err)
	}

	return length, nil
}

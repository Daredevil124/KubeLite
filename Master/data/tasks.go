package data

import (
	"context"
	"log"

	"github.com/redis/go-redis/v9"
)

// would be used by worker node
func incrementTask() {
	ctx := context.Background()
	err := RedisClient.Incr(ctx, "task_completed").Err() //variable name task_Completed get incremented by 1
	if err != nil {
		log.Printf("Failed to count Task %v", err)
	}
}

// would be used my master node
func getTotal_Task() (int64, error) {
	ctx := context.Background()
	completed_task, err := RedisClient.Get(ctx, "task_completed").Int64() //completed task holds all the task completed by all workers
	if err == redis.Nil {
		return 0, nil
	} else if err != nil {
		log.Printf("Failed to read from redis %v", err)
		return 0, err
	} else {
		return completed_task, nil
	}
}

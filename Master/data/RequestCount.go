package data

import (
	"context"
	"log"

	"github.com/redis/go-redis/v9"
)

func IncrementRequestCount() { //first letter capital makes this func public
	ctx := context.Background()
	err := RedisClient.Incr(ctx, "total_request").Err() //variable name total_request get incremented by 1
	if err != nil {
		log.Printf("Failed to count request %v", err)
	}
}
func GetRequestCount() (uint64, error) {
	ctx := context.Background()
	completed_task, err := RedisClient.Get(ctx, "total_request").Int64() //completed task holds all the request
	if err == redis.Nil {
		return 0, nil
	} else if err != nil {
		log.Printf("Failed to read from redis %v", err)
		return 0, err
	} else {
		return uint64(completed_task), nil
	}
}

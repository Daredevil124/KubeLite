package main

import (
	"os"

	"github.com/redis/go-redis/v9"
)

var rdb *redis.Client // This variable will hold our active Redis connection pool so worker functions across the package can reuse it.

// InitRedisClient initializes the Redis client for the Worker node.
func InitRedisClient() *redis.Client {
	redisAddr := os.Getenv("REDIS_ADDR")

	if redisAddr == "" {
		redisAddr = "host.docker.internal:6379" // default addr
	}

	client := redis.NewClient(&redis.Options{
		Addr:     redisAddr,
		Password: "", // No password set by default
		DB:       0,  // Use default DB
	})

	return client
}

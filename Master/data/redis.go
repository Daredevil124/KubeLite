package data

import (
	"os"

	"github.com/redis/go-redis/v9"
)

var RedisClient *redis.Client

func InitRedis() {
	redisAddr := os.Getenv("REDIS_ADDR")
	if redisAddr == "" {
		redisAddr = "127.0.0.1:6379"
	}

	RedisClient = redis.NewClient(&redis.Options{
		Addr:     redisAddr,
		Password: "",
		DB:       0, // Default bucket 0
	})
}

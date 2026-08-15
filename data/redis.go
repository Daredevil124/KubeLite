package data

import "github.com/redis/go-redis/v9"

var RedisClient *redis.Client

func initRedis() {
	RedisClient = redis.NewClient(&redis.Options{
		Password: "",
		DB:       0, //Defualt bucket 0
	})
}

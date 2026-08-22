package data

import "github.com/redis/go-redis/v9"

var RedisClient *redis.Client

func InitRedis() {
	RedisClient = redis.NewClient(&redis.Options{
		Password: "",
		DB:       0, //Defualt bucket 0
	})
}

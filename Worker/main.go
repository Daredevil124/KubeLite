package main

import (
	"context"
	"fmt"
	"os"

	"github.com/redis/go-redis/v9"
)

var rdb *redis.Client // This variable will hold our active Redis connection pool so worker functions across the package can reuse it.

// initRedisClient initializes the Redis client for the Worker node.
// Note: When running inside a Docker container, 'localhost:6379' points to the container itself.
// to reach the Master's Redis queue running on the host, we configure the connection to point
// to 'host.docker.internal:6379' (or the REDIS_ADDR environment variable if overridden).
func InitRedisClient() *redis.Client {
	redisAddr := os.Getenv("REDIS_ADDR") // checks whether the redis_addr name is passed into the docker container
	// also this is a global check (os.getenv), it could check either the local pc, docker, kubernetes, etc. Applications should NOT care where they run, hence we used os.getenv here.

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

func main() {
	fmt.Println("Worker Node Starting...")

	rdb = InitRedisClient() // the returned client pointer is stored in the rdb variable
	ctx := context.Background()

	// Verify Redis connectivity
	pong, err := rdb.Ping(ctx).Result()
	if err != nil {
		fmt.Printf(" Worker failed to connect to Redis at %s: %v\n", rdb.Options().Addr, err)
	} else {
		fmt.Printf("Worker successfully connected to Redis at %s: %s\n", rdb.Options().Addr, pong)
	}

	// Worker queue polling logic will be executed here
}

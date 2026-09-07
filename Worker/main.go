package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/redis/go-redis/v9"
)

// TaskPayload represents the JSON structure expected from Redis
type TaskPayload struct {
	Type  string `json:"type"`
	Value int    `json:"value"`
}

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

	fmt.Println("Worker is now waiting for tasks on 'task_queue'...")
	
	for {
		// BLPop blocks until an element is available. A timeout of 0 means block indefinitely.
		// This allows the worker to consume 0% CPU while sitting idle.
		result, err := rdb.BLPop(ctx, 0, "task_queue").Result()
		if err != nil {
			fmt.Printf("Error retrieving task from queue: %v\n", err)
			continue
		}

		// result[0] is the queue name ("task_queue")
		// result[1] is the actual task payload
		payload := result[1]
		fmt.Printf("Received task payload: %s\n", payload)
		
		var task TaskPayload
		err = json.Unmarshal([]byte(payload), &task)
		if err != nil {
			fmt.Printf("Failed to parse JSON: %v\n", err)
			continue
		}

		switch task.Type {
		case "prime":
			res := FindNthPrime(task.Value)
			fmt.Printf("Result of FindNthPrime(%d) = %d\n", task.Value, res)
		case "is_power_of_two":
			res := IsPowerOfTwo(task.Value)
			fmt.Printf("Result of IsPowerOfTwo(%d) = %v\n", task.Value, res)
		case "stress_cpu":
			fmt.Printf("Starting CPU stress test for %d seconds...\n", task.Value)
			StressCPUAllCores(task.Value)
			fmt.Println("CPU stress test completed.")
		default:
			fmt.Printf("Unknown task type received: %s\n", task.Type)
		}
	}
}

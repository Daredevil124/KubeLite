package main

import (
	"Worker/tasks"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"

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
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Graceful shutdown: listen for SIGTERM (sent by Master via ContainerStop).
	// The signal goroutine sets the flag and cancels ctx to unblock BLPop immediately.
	var isShuttingDown atomic.Bool
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, os.Interrupt)
	go func() {
		<-sigCh
		fmt.Println("Worker: SIGTERM received — finishing current task then exiting cleanly...")
		isShuttingDown.Store(true)
		cancel() // unblocks the BLPop call
	}()

	// Verify Redis connectivity
	pong, err := rdb.Ping(ctx).Result()
	if err != nil {
		fmt.Printf(" Worker failed to connect to Redis at %s: %v\n", rdb.Options().Addr, err)
	} else {
		fmt.Printf("Worker successfully connected to Redis at %s: %s\n", rdb.Options().Addr, pong)
	}

	fmt.Println("Worker is now waiting for tasks on 'task_queue'...")
	
	for {
		// BLMove atomically pops an element from 'task_queue' and pushes it to 'processing_queue'.
		// 0 timeout means it blocks indefinitely, consuming 0% CPU while idle.
		// "LEFT" means we pop from the front of the line, "RIGHT" means we add it to the back of the processing queue.
		payload, err := rdb.BLMove(ctx, "task_queue", "processing_queue", "LEFT", "RIGHT", 0).Result()

		// If the shutdown flag is set and BLMove returned an error (e.g. context canceled while idle), exit cleanly.
		if isShuttingDown.Load() && err != nil {
			fmt.Println("Worker: shutdown complete — exiting.")
			os.Exit(0)
		}

		if err != nil {
			if isShuttingDown.Load() {
				fmt.Println("Worker: shutdown complete — exiting.")
				os.Exit(0)
			}
			fmt.Printf("Error retrieving task from queue: %v\n", err)
			continue
		}

		fmt.Printf("Received task payload and safely moved to processing_queue: %s\n", payload)

		// Independent context for cleanup operations so Redis commands (Incr, LRem)
		// succeed even if 'ctx' was canceled by a SIGTERM received during task execution.
		cleanupCtx := context.Background()

		var task TaskPayload
		err = json.Unmarshal([]byte(payload), &task)
		if err != nil {
			fmt.Printf("Corrupted JSON payload received from queue: %v. Removing from processing_queue and skipping.\n", err)
			rdb.LRem(cleanupCtx, "processing_queue", 1, payload)
			if isShuttingDown.Load() {
				fmt.Println("Worker: shutdown complete — exiting.")
				os.Exit(0)
			}
			continue
		}

		switch task.Type {
		case "prime":
			res := tasks.FindNthPrime(task.Value)
			fmt.Printf("Result of FindNthPrime(%d) = %d\n", task.Value, res)
			rdb.Incr(cleanupCtx, "task_completed")
		case "is_power_of_two":
			res := tasks.IsPowerOfTwo(task.Value)
			fmt.Printf("Result of IsPowerOfTwo(%d) = %v\n", task.Value, res)
			rdb.Incr(cleanupCtx, "task_completed")
		case "stress_cpu":
			fmt.Printf("Starting CPU stress test for %d seconds...\n", task.Value)
			tasks.StressCPUAllCores(task.Value)
			fmt.Println("CPU stress test completed.")
			rdb.Incr(cleanupCtx, "task_completed")
		default:
			fmt.Printf("Unknown task type received: %s\n", task.Type)
		}

		// Task finished — remove it from processing_queue so the Master's
		// recovery loop doesn't mistake it for a crashed-worker task and re-queue it.
		// LREM count=1 removes the first (and only) occurrence of this exact payload.
		rdb.LRem(cleanupCtx, "processing_queue", 1, payload)

		// If SIGTERM was received during task processing, exit cleanly now after cleanup.
		if isShuttingDown.Load() {
			fmt.Println("Worker: finished processing current task after SIGTERM — exiting cleanly.")
			os.Exit(0)
		}
	}
}

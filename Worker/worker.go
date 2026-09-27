package main

import (
	"Worker/tasks"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync/atomic"
)

// StartPolling runs the infinite loop pulling tasks from Redis.
func StartPolling(ctx context.Context, isShuttingDown *atomic.Bool) {
	for {
		// BLPop blocks until an element is available. 0 timeout means block indefinitely.
		result, err := rdb.BLPop(ctx, 0, "task_queue").Result()

		// If the shutdown flag is set and BLPop returned an error (context canceled), exit cleanly.
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

		rawPayload := result[1] // result[0] is the queue name

		// 1. Grab Container ID (Injected by Docker)
		containerID := os.Getenv("HOSTNAME")

		// 2. Wrap the payload to perfectly match Master's TaskPair expectation
		wrappedTask := TaskPair{
			First:  containerID,
			Second: rawPayload,
		}

		// 3. Marshal it into a JSON string
		wrappedBytes, err := json.Marshal(wrappedTask)
		if err != nil {
			fmt.Printf("Failed to marshal wrapped task: %v\n", err)
			continue
		}
		wrappedPayload := string(wrappedBytes)

		// 4. Manually push this wrapped payload to processing_queue
		cleanupCtx := context.Background()
		rdb.RPush(cleanupCtx, "processing_queue", wrappedPayload)

		fmt.Printf("Received task payload and manually registered in processing_queue: %s\n", rawPayload)

		var task TaskPayload
		err = json.Unmarshal([]byte(rawPayload), &task)
		if err != nil {
			fmt.Printf("Corrupted JSON payload received from queue: %v. Removing from processing_queue and skipping.\n", err)
			rdb.LRem(cleanupCtx, "processing_queue", 1, wrappedPayload)
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

		// 5. Cleanup: Remove the WRAPPED payload from processing_queue
		rdb.LRem(cleanupCtx, "processing_queue", 1, wrappedPayload)

		// If SIGTERM was received during task processing, exit cleanly now after cleanup.
		if isShuttingDown.Load() {
			fmt.Println("Worker: finished processing current task after SIGTERM — exiting cleanly.")
			os.Exit(0)
		}
	}
}

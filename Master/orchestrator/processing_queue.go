package orchestrator

import (
	"KubeLite/data"
	"context"
	"encoding/json"
	"log"

	"github.com/docker/docker/api/types/container"
	"github.com/redis/go-redis/v9"
)

type TaskPair struct {
	First  string `json:"first"`
	Second string `json:"second"`
}

func Processing_to_main(currentWorkers []container.Summary) {
	ctx := context.Background()
	mp := make(map[string]bool)
	for _, c := range currentWorkers { //using map for O(logn) lookup for container ID
		mp[c.ID] = true
		if len(c.ID) >= 12 {
			mp[c.ID[:12]] = true //if the worker nodes store 12 byte rather than 64 bytes
		}
	}
	for {
		rawTask, err := data.RedisClient.LPop(ctx, "processing_queue").Result() //popping from left
		if err == redis.Nil {
			// processing_queue is completely empty; filtering is done
			break
		} else if err != nil {
			log.Printf("Redis pop error: %v", err)
			break
		}
		var task TaskPair
		if err := json.Unmarshal([]byte(rawTask), &task); err != nil { //unmarshalling the data into a pair
			log.Printf("Corrupted JSON in processing_queue, skipping: %v", err)
			continue // Avoid pushing broken items to task_queue
		}
		if mp[task.First] { //checking if the ID exists in the current Worker nodes
			data.RedisClient.RPush(ctx, "temp_queue", rawTask) //pushing into a temporary queue
		} else {
			task.First = ""                      //emptying the ID
			cleanJSON, err := json.Marshal(task) //marshalling back
			if err != nil {
				log.Printf("Failed to marshal sanitized task: %v", err)
				continue
			}
			data.RedisClient.RPush(ctx, "task_queue", cleanJSON) //If ID not present shift it to the main queue. This happens if the worker node crashes before finishing the task
		}
	}
	for { //tranfer all the data from temporary queue to processing_queue
		err := data.RedisClient.LMove(ctx, "temp_queue", "processing_queue", "LEFT", "RIGHT").Err()
		if err == redis.Nil {
			// Queue is empty, restore complete!
			break
		} else if err != nil {
			log.Printf("Error restoring deferred queue: %v", err)
			break
		}
	}
}

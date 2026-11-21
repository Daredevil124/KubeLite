package orchestrator

import (
	"KubeLite/data"
	"context"
	"testing"

	"github.com/docker/docker/api/types/container"
)

func TestProcessingToMainAtomicRescue(t *testing.T) {
	data.InitRedis()
	ctx := context.Background()

	// Verify Redis connection; skip if redis is not running
	if err := data.RedisClient.Ping(ctx).Err(); err != nil {
		t.Skip("Redis not available on localhost:6379, skipping integration test")
	}

	// Clean up test keys
	data.RedisClient.Del(ctx, "processing_queue", "task_queue", "temp_queue")
	defer data.RedisClient.Del(ctx, "processing_queue", "task_queue", "temp_queue")

	fullAliveID := "a1b2c3d4e5f67890123456789012345678901234567890123456789012345678"
	shortHostnameAliveID := "a1b2c3d4e5f6" // container $HOSTNAME inside worker
	deadWorkerID := "deadbeef9999"

	// 1. Seed processing_queue with:
	//    - Active worker task (using 12-char short ID)
	//    - Dead worker task
	//    - Corrupted JSON string
	aliveTaskJSON := `{"first":"` + shortHostnameAliveID + `","second":"{\"type\":\"prime\",\"value\":50000}"}`
	deadTaskJSON := `{"first":"` + deadWorkerID + `","second":"{\"type\":\"stress_cpu\",\"value\":10}"}`
	corruptJSON := `not-a-valid-json`

	if err := data.RedisClient.RPush(ctx, "processing_queue", aliveTaskJSON, deadTaskJSON, corruptJSON).Err(); err != nil {
		t.Fatalf("failed to seed processing_queue: %v", err)
	}

	// 2. Mock currentWorkers with full 64-char ID
	mockWorkers := []container.Summary{
		{
			ID: fullAliveID,
		},
	}

	// 3. Run Processing_to_main
	Processing_to_main(mockWorkers)

	// 4. Verify that alive task is STILL in processing_queue (never moved, never lost, matched via 12-char prefix)
	procItems, err := data.RedisClient.LRange(ctx, "processing_queue", 0, -1).Result()
	if err != nil {
		t.Fatalf("failed to read processing_queue: %v", err)
	}
	if len(procItems) != 1 {
		t.Fatalf("expected 1 item in processing_queue, got %d: %v", len(procItems), procItems)
	}
	if procItems[0] != aliveTaskJSON {
		t.Errorf("expected alive task %q, got %q", aliveTaskJSON, procItems[0])
	}

	// 5. Verify that dead worker's task was rescued into task_queue
	taskQueueItems, err := data.RedisClient.LRange(ctx, "task_queue", 0, -1).Result()
	if err != nil {
		t.Fatalf("failed to read task_queue: %v", err)
	}
	if len(taskQueueItems) != 1 {
		t.Fatalf("expected 1 item in task_queue, got %d: %v", len(taskQueueItems), taskQueueItems)
	}
	expectedPayload := `{"type":"stress_cpu","value":10}`
	if taskQueueItems[0] != expectedPayload {
		t.Errorf("expected rescued payload %q, got %q", expectedPayload, taskQueueItems[0])
	}

	// 6. Verify that corrupt JSON was cleaned up from processing_queue and NOT pushed to task_queue
	for _, item := range taskQueueItems {
		if item == corruptJSON {
			t.Errorf("corrupt item leaked into task_queue: %s", item)
		}
	}

	// 7. Verify that temp_queue remains 0 (no longer used)
	tempLen, err := data.RedisClient.LLen(ctx, "temp_queue").Result()
	if err != nil {
		t.Fatalf("failed to read temp_queue length: %v", err)
	}
	if tempLen != 0 {
		t.Errorf("expected temp_queue to be 0, got %d", tempLen)
	}
}

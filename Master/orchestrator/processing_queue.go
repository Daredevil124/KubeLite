package orchestrator

import (
	"KubeLite/data"
	"context"
	"encoding/json"
	"log"
	"strings"

	"github.com/docker/docker/api/types/container"
	"github.com/redis/go-redis/v9"
)

type TaskPair struct {
	First  string `json:"first"`
	Second string `json:"second"`
}

// rescueOrphanTasksScript inspects processing_queue in-place WITHOUT moving active tasks
// into a temporary queue (avoiding race conditions where a worker finishes and LRem removes 0 elements).
// Only tasks belonging to dead/non-existent workers are atomically removed and restored to task_queue.
var rescueOrphanTasksScript = redis.NewScript(`
local active_raw = cjson.decode(ARGV[1])
local active = {}
for _, id in ipairs(active_raw) do
    active[string.lower(id)] = true
end

local tasks = redis.call('LRANGE', KEYS[1], 0, -1)
local rescued = 0

for _, raw in ipairs(tasks) do
    local ok, item = pcall(cjson.decode, raw)
    if ok and type(item) == "table" and item.first and item.first ~= "" then
        local first_lower = string.lower(item.first)
        if not active[first_lower] then
            local removed = redis.call('LREM', KEYS[1], 1, raw)
            if removed > 0 and item.second and item.second ~= "" then
                redis.call('RPUSH', KEYS[2], item.second)
                rescued = rescued + 1
            end
        end
    else
        -- Clean up corrupted JSON payloads or missing container ID so processing_queue does not accumulate junk
        redis.call('LREM', KEYS[1], 1, raw)
    end
end

return rescued
`)

func Processing_to_main(currentWorkers []container.Summary) {
	ctx := context.Background()
	if data.RedisClient == nil {
		return
	}

	// Drain any legacy items from temp_queue if left over from previous versions
	for {
		err := data.RedisClient.LMove(ctx, "temp_queue", "processing_queue", "LEFT", "RIGHT").Err()
		if err == redis.Nil || err != nil {
			break
		}
	}

	// Collect active container IDs (both full 64-char ID and standard 12-char hostname prefix)
	activeIDs := make([]string, 0, len(currentWorkers)*2)
	for _, c := range currentWorkers {
		cID := strings.ToLower(c.ID)
		activeIDs = append(activeIDs, cID)
		if len(cID) >= 12 {
			activeIDs = append(activeIDs, cID[:12])
		}
	}

	activeJSON, err := json.Marshal(activeIDs)
	if err != nil {
		log.Printf("Failed to marshal active worker IDs: %v", err)
		return
	}

	res, err := rescueOrphanTasksScript.Run(ctx, data.RedisClient, []string{"processing_queue", "task_queue"}, string(activeJSON)).Int()
	if err != nil && err != redis.Nil {
		log.Printf("Error running rescueOrphanTasksScript: %v", err)
		return
	}

	if res > 0 {
		log.Printf("Rescued %d orphaned task(s) from dead worker(s) and restored to task_queue", res)
	}
}

package main

// TaskPayload represents the JSON structure expected from Redis
type TaskPayload struct {
	Type  string `json:"type"`
	Value int    `json:"value"`
}

// TaskPair wraps the payload with the Container ID for the Master's processing_queue.
type TaskPair struct {
	First  string `json:"first"`
	Second string `json:"second"`
}

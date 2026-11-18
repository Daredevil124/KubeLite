package main

// TaskPayload represents the JSON structure accepted from Redis.
// Supports both the internal worker schema {"type": "...", "value": ...}
// and the public API/README schema {"task_type": "...", "target": ...} or {"n": ...}.
type TaskPayload struct {
	Type     string `json:"type"`
	TaskType string `json:"task_type"`
	Value    int    `json:"value"`
	Target   int    `json:"target"`
	N        int    `json:"n"`
}

// Normalize ensures backwards-compatibility across different client schemas:
// - Maps task_type -> type ("prime_computation" or "prime" -> "prime", etc.)
// - Maps target / n -> value
// - Sets sensible defaults if value is missing or non-positive
func (t *TaskPayload) Normalize() {
	taskType := t.Type
	if taskType == "" {
		taskType = t.TaskType
	}

	switch taskType {
	case "prime", "prime_computation":
		t.Type = "prime"
	case "is_power_of_two":
		t.Type = "is_power_of_two"
	case "stress_cpu":
		t.Type = "stress_cpu"
	default:
		// If type wasn't explicitly provided but target or n was given, default to prime
		if t.Target > 0 || t.N > 0 {
			t.Type = "prime"
		}
	}

	if t.Value == 0 {
		if t.Target != 0 {
			t.Value = t.Target
		} else if t.N != 0 {
			t.Value = t.N
		}
	}

	// Sensible defaults for non-positive values
	if t.Value <= 0 {
		switch t.Type {
		case "prime":
			t.Value = 50000
		case "stress_cpu":
			t.Value = 5
		case "is_power_of_two":
			t.Value = 1024
		}
	}
}

// TaskPair wraps the payload with the Container ID for the Master's processing_queue.
type TaskPair struct {
	First  string `json:"first"`
	Second string `json:"second"`
}


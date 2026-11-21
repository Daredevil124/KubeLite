package controller

import (
	"KubeLite/data"
	"bytes"
	"context"
	"io"
	"net/http"
)

// what happens when a user submits a task to KubeLite.

func Request(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	w.Header().Set("Content-Type", "application/json")
	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"Method not allowed, use POST"}`, http.StatusMethodNotAllowed)
		return
	}

	defer r.Body.Close()

	body, err := io.ReadAll(r.Body) //pulls all the data from r.Body and into body
	if err != nil {
		http.Error(w, "Failed to read the body", http.StatusBadRequest)
		return
	}

	trimmedBody := bytes.TrimSpace(body)
	if len(trimmedBody) == 0 {
		http.Error(w, `{"error":"Payload cannot be empty"}`, http.StatusBadRequest)
		return
	}

	data.IncrementRequestCount() // basically request++

	ctx := context.Background()
	err = data.RedisClient.RPush(ctx, "task_queue", string(trimmedBody)).Err() //pushes at the end/right. ListName=task_queue and return error if it occurs
	if err != nil {
		http.Error(w, "Failed to push to the queue", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusAccepted)
	w.Write([]byte(`{"status":"queued successfully"}`))
}

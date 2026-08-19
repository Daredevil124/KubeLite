package controller

import (
	"KubeLite/data"
	"context"
	"io"
	"net/http"
)

func Request(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")            //allow http request from any origin
	w.Header().Set("Access-Control-Allow-Method", "POST-OPTIONS") //allow post method
	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}
	data.IncrementRequestCount()
	ctx := context.Background()
	body, err := io.ReadAll(r.Body) //pulls all the data from r.Body and into body
	if err != nil {
		http.Error(w, "Failed to read the body", http.StatusBadRequest)
		return
	}
	err = data.RedisClient.RPush(ctx, "task_queue", string(body)).Err() //pushes at the end/right. ListName=task_queue and return error if it occurs
	if err != nil {
		http.Error(w, "Failed to push to the queue", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusAccepted)
	w.Write([]byte(`{"status:queued successfully"}`))
}

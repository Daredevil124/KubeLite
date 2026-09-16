package controller

import (
	"KubeLite/data"
	"KubeLite/metrics"
	"encoding/json"
	"net/http"
	"time"
)

// MetricsResponse is the JSON shape returned by GET /metrics
type MetricsResponse struct {
	CPUPercent    float64   `json:"cpu_percent"`
	MemoryPercent float64   `json:"memory_percent"`
	QueueLength   int64     `json:"queue_length"`
	TotalRequests uint64    `json:"total_requests"`
	TotalTasks    int64     `json:"total_tasks"`
	WorkerCount   int       `json:"worker_count"`
	Timestamp     time.Time `json:"timestamp"`
}

// GetMetrics handles GET /metrics — returns a live JSON snapshot of cluster state.
// The frontend polls this every 2 seconds to drive the dashboard.
func GetMetrics(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Content-Type", "application/json")

	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	cpu, _ := metrics.GetTotalClusterCPU()
	memory, _ := metrics.GetTotalClusterMemory()
	queue, _ := metrics.GetQueueLength()
	workerCount, _ := metrics.GetWorkerCount()
	requests, _ := data.GetRequestCount()
	tasks, _ := data.GetTotal_Task()

	resp := MetricsResponse{
		CPUPercent:    cpu,
		MemoryPercent: memory,
		QueueLength:   queue,
		TotalRequests: requests,
		TotalTasks:    tasks,
		WorkerCount:   workerCount,
		Timestamp:     time.Now(),
	}

	json.NewEncoder(w).Encode(resp)
}

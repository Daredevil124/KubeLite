package main

import (
	"KubeLite/controller"
	"KubeLite/data"
	"KubeLite/orchestrator"
	"log"
	"net/http"
)

func main() {
	data.InitRedis()
	go orchestrator.Orchestrate() // background: collect metrics every 10s

	http.HandleFunc("/evaluate", controller.Request)    // POST — evaluate workload (Architecture spec)
	http.HandleFunc("/request", controller.Request)     // POST — submit a task (alias)
	http.HandleFunc("/metrics", controller.GetMetrics)  // GET  — live cluster stats

	log.Println("Master Node listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

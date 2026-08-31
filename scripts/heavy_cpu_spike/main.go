package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"sync/atomic"
	"time"
)

const (
	masterURL   = "http://localhost:8080/request"
	numRequests = 100
	concurrency = 10
)

type TaskPayload struct {
	Type  string `json:"type"`
	Value int    `json:"value"`
}

func sendHeavyRequest(client *http.Client) bool {
	// underscore here is basically containing error but since we dont use error here, to avoid unnecessary syntax problem we use _
	payload, _ := json.Marshal(TaskPayload{ //json.Marshal takes a Go struct and converts ("marshals") it into a JSON byte array ([]byte)
		Type:  "prime",
		Value: 100000,
	})
	// Input: TaskPayload{Type: "prime", Value: 100000}
	// Output: []byte('{"type":"prime","value":100000}')

	// http.NewRequest("POST", masterURL, body) expects its 3rd argument (body) to be an io.Reader (a data stream).
	// payload is a raw byte slice ([]byte) sitting in memory.
	// bytes.NewBuffer(payload) wraps that raw byte slice into an io.Reader stream object so http.NewRequest can read data from it like a network stream.
	req, err := http.NewRequest("POST", masterURL, bytes.NewBuffer(payload))
	if err != nil {
		return false
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req) //client.Do(req) executes the HTTP request over the network.
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	return resp.StatusCode == http.StatusAccepted || resp.StatusCode == http.StatusOK
}

func main() {
	fmt.Println("[Heavy-Heavy] Starting CPU Spike Load Test in Go...")
	fmt.Printf("Sending %d heavy tasks across %d parallel goroutines to %s...\n\n", numRequests, concurrency, masterURL)

	client := &http.Client{Timeout: 5 * time.Second}
	startTime := time.Now()

	var successCount uint64
	var wg sync.WaitGroup
	semaphore := make(chan struct{}, concurrency)

	for i := 0; i < numRequests; i++ {
		wg.Add(1)               // add 1 to the wait group, we will keep on adding it until it reaches 100
		semaphore <- struct{}{} //The for loop pauses and will NOT execute go func() for request #11 yet.
		go func() {
			defer wg.Done()                // Done decrements the wait group, "1 task finished, 99 remaining"
			defer func() { <-semaphore }() // this increments the semaphore value by 1, meaning the task is completed.
			if sendHeavyRequest(client) {
				atomic.AddUint64(&successCount, 1)
			}
		}()
	}

	wg.Wait() // This unblocks the wait group, this means all the tasks are done and program can exit.
	elapsed := time.Since(startTime).Seconds()

	fmt.Printf("\n[Heavy-Heavy Test Completed in %.2fs]\n", elapsed)
	fmt.Printf("Successfully queued %d/%d heavy CPU tasks.\n", successCount, numRequests)
	fmt.Println("Check Master logs/dashboard: Worker CPU usage should be ~100%, triggering scale-up!")
}

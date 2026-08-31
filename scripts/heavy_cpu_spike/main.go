package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math/rand"
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

// TaskPayload represents the JSON sent to Master node
type TaskPayload struct {
	Type  string `json:"type"`
	Value int    `json:"value"`
}

// Counter tracking generated task types
var (
	primeTaskCount     uint64
	stressCPUTaskCount uint64
	powerOfTwoCount    uint64
)

// generateProbabilisticTask uses Gaussian (Normal) distribution sampling
// centered at mean = 0.8, stddev = 0.15 to probabilistically choose between
// heavy tasks (prime / stress_cpu) and light tasks (is_power_of_two).
func generateProbabilisticTask() TaskPayload {
	// Gaussian sample centered at 0.8
	sample := rand.NormFloat64()*0.15 + 0.8

	if sample >= 0.4 {
		// Heavy CPU Workload (80-90% probability)
		if rand.Float64() < 0.6 {
			atomic.AddUint64(&primeTaskCount, 1)
			return TaskPayload{
				Type:  "prime",
				Value: 80000 + rand.Intn(40000), // Find 80,000th to 120,000th prime
			}
		} else {
			atomic.AddUint64(&stressCPUTaskCount, 1)
			return TaskPayload{
				Type:  "stress_cpu",
				Value: 5 + rand.Intn(10), // Burn CPU for 5-15 seconds
			}
		}
	} else {
		// Light CPU Workload (10-20% probability)
		atomic.AddUint64(&powerOfTwoCount, 1)
		return TaskPayload{
			Type:  "is_power_of_two",
			Value: 1024 + rand.Intn(10000),
		}
	}
}

func sendRequest(client *http.Client) bool {
	// Generate probabilistic task selection (Gaussian distribution)
	task := generateProbabilisticTask()

	payload, err := json.Marshal(task) //json.Marshal takes a Go struct and converts ("marshals") it into a JSON byte array ([]byte)

	if err != nil {
		return false
	}

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
	rand.Seed(time.Now().UnixNano()) // Seed pseudo-random generator

	fmt.Println("[Heavy-Heavy] Starting Probabilistic CPU Spike Load Test in Go...")
	fmt.Printf("Sending %d tasks using Gaussian probability distribution (concurrency=%d)...\n\n", numRequests, concurrency)

	client := &http.Client{Timeout: 5 * time.Second}
	startTime := time.Now()

	var successCount uint64
	var wg sync.WaitGroup
	semaphore := make(chan struct{}, concurrency)

	for i := 0; i < numRequests; i++ {
		wg.Add(1)               // add 1 to the wait group, we will keep on adding it until it reaches 100
		semaphore <- struct{}{} // Pause loop if 10 concurrent requests are running
		go func() {
			defer wg.Done()                // Done decrements the wait group, "1 task finished, 99 remaining"
			defer func() { <-semaphore }() // Release semaphore slot when task finishes
			if sendRequest(client) {
				atomic.AddUint64(&successCount, 1)
			}
		}()
	}

	wg.Wait() // This unblocks the wait group, this means all the tasks are done and program can exit.
	elapsed := time.Since(startTime).Seconds()

	fmt.Printf("\n[Heavy-Heavy Test Completed in %.2fs]\n", elapsed)
	fmt.Printf("Successfully queued %d/%d tasks.\n", successCount, numRequests)
	fmt.Println("\n Task Probability Distribution Generated:")
	fmt.Printf("  • Prime Calculations (Heavy):     %d tasks\n", atomic.LoadUint64(&primeTaskCount))
	fmt.Printf("  • Multi-Core Stress (Heavy):     %d tasks\n", atomic.LoadUint64(&stressCPUTaskCount))
	fmt.Printf("  • Power Of Two Checks (Light):    %d tasks\n", atomic.LoadUint64(&powerOfTwoCount))
	fmt.Println("\nCheck Master logs/dashboard: Worker CPU usage should peg near ~100%, triggering scale-up!")
}

//Note :  Key Takeaway: Gaussian determines what task to create, while Poisson determines when to send it!

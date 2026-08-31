package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

const (
	masterURL         = "http://localhost:8080/request"
	concurrency       = 50
	arrivalRateLambda = 25.0 // High Poisson arrival rate (25 req/sec) to trigger Queue Backlog Spike
)

type TaskPayload struct {
	Type  string `json:"type"`
	Value int    `json:"value"`
}

// Counters tracking task probability distribution
var (
	powerOfTwoCount    uint64
	primeTaskCount     uint64
	stressCPUTaskCount uint64
	totalSentCount     uint64
	successCount       uint64
)

// generateProbabilisticTask uses Gaussian distribution sampling centered at mean = 0.5
// to achieve: 50% light tasks (is_power_of_two), 40% medium tasks (prime), 10% heavy tasks (stress_cpu).
func generateProbabilisticTask() TaskPayload {
	// Gaussian sample centered at 0.5
	sample := rand.NormFloat64()*0.20 + 0.5

	if sample < 0.50 {
		// Light CPU Workload (50% probability)
		atomic.AddUint64(&powerOfTwoCount, 1)
		return TaskPayload{
			Type:  "is_power_of_two",
			Value: 1024 + rand.Intn(10000),
		}
	} else {
		// Remaining 50%: 40% overall for prime (80% of remaining) vs 10% overall for stress_cpu (20% of remaining)
		if rand.Float64() < 0.85 {
			atomic.AddUint64(&primeTaskCount, 1)
			return TaskPayload{
				Type:  "prime",
				Value: 40000 + rand.Intn(20000),
			}
		} else {
			atomic.AddUint64(&stressCPUTaskCount, 1)
			return TaskPayload{
				Type:  "stress_cpu",
				Value: 3 + rand.Intn(5),
			}
		}
	}
}

func sendRequest(client *http.Client) bool {
	task := generateProbabilisticTask()
	fmt.Printf(" [Queue Task Sent] Type: %-15s | Value: %-6d\n", task.Type, task.Value)

	payload, err := json.Marshal(task)
	if err != nil {
		return false
	}

	req, err := http.NewRequest("POST", masterURL, bytes.NewBuffer(payload))
	if err != nil {
		return false
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	return resp.StatusCode == http.StatusAccepted || resp.StatusCode == http.StatusOK
}

func main() {
	rand.Seed(time.Now().UnixNano()) // Seed pseudo-random generator

	fmt.Println(" [Light-Heavy] Starting Continuous Queue Backlog Spike Load Test in Go...")
	fmt.Printf("Running infinitely with High Poisson arrival rate (λ=%.1f req/sec, concurrency=%d)...\n", arrivalRateLambda, concurrency)
	fmt.Println("Press Ctrl+C to stop the load test and view final report.")

	client := &http.Client{Timeout: 3 * time.Second}
	startTime := time.Now()

	// Capture Ctrl+C signal for graceful shutdown report
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGINT)

	var wg sync.WaitGroup
	semaphore := make(chan struct{}, concurrency)
	stopChan := make(chan struct{})

	// Handle graceful shutdown on Ctrl+C
	go func() {
		<-sigChan
		fmt.Println("\n\nStopping queue backlog load generator...")
		close(stopChan)
	}()

	// Infinite Poisson request generation loop
loop:
	for {
		select {
		case <-stopChan:
			break loop
		default:
			// Calculate Poisson process inter-arrival delay (exponential distribution)
			poissonDelay := time.Duration((rand.ExpFloat64() / arrivalRateLambda) * float64(time.Second))
			time.Sleep(poissonDelay)

			atomic.AddUint64(&totalSentCount, 1)
			wg.Add(1)
			semaphore <- struct{}{}
			go func() {
				defer wg.Done()
				defer func() { <-semaphore }()
				if sendRequest(client) {
					atomic.AddUint64(&successCount, 1)
				}
			}()
		}
	}

	wg.Wait()
	elapsed := time.Since(startTime).Seconds()

	fmt.Printf("\n [Queue Backlog Load Test Summary - Ran for %.2fs]\n", elapsed)
	fmt.Printf("Total Sent: %d | Successfully Queued: %d (%.1f req/sec)\n",
		atomic.LoadUint64(&totalSentCount),
		atomic.LoadUint64(&successCount),
		float64(atomic.LoadUint64(&successCount))/elapsed)
	fmt.Println("\n Task Probability Distribution Generated:")
	fmt.Printf("  • Power Of Two Checks (Light ~50%%):  %d tasks\n", atomic.LoadUint64(&powerOfTwoCount))
	fmt.Printf("  • Prime Calculations (Medium ~40%%):  %d tasks\n", atomic.LoadUint64(&primeTaskCount))
	fmt.Printf("  • Multi-Core Stress (Heavy ~10%%):    %d tasks\n", atomic.LoadUint64(&stressCPUTaskCount))
	fmt.Println("\n Check Master logs/dashboard: Redis Queue backlog should explode, triggering queue-based scale-up!")
}

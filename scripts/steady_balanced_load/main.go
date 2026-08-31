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
	concurrency       = 15
	arrivalRateLambda = 4.0 // Steady Poisson arrival rate (4 req/sec) to simulate normal operational load
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
// to achieve a perfectly balanced 3-way distribution:
// ~33% Light (is_power_of_two), ~33% Medium (prime), ~34% Heavy (stress_cpu).
func generateProbabilisticTask() TaskPayload {
	sample := rand.NormFloat64()*0.20 + 0.5

	if sample < 0.38 {
		// Light CPU Workload (~33% probability)
		atomic.AddUint64(&powerOfTwoCount, 1)
		return TaskPayload{
			Type:  "is_power_of_two",
			Value: 1024 + rand.Intn(10000),
		}
	} else {
		// Remaining 67%: ~33% overall for prime vs ~34% overall for stress_cpu
		if rand.Float64() < 0.50 {
			atomic.AddUint64(&primeTaskCount, 1)
			return TaskPayload{
				Type:  "prime",
				Value: 30000 + rand.Intn(20000),
			}
		} else {
			atomic.AddUint64(&stressCPUTaskCount, 1)
			return TaskPayload{
				Type:  "stress_cpu",
				Value: 3 + rand.Intn(4),
			}
		}
	}
}

func sendRequest(client *http.Client) bool {
	task := generateProbabilisticTask()
	fmt.Printf(" [Steady Task Sent] Type: %-15s | Value: %-6d\n", task.Type, task.Value)

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

	fmt.Println(" [Balanced Load] Starting Continuous Steady State Load Test in Go...")
	fmt.Printf("Running infinitely with Steady Poisson arrival rate (λ=%.1f req/sec, concurrency=%d)...\n", arrivalRateLambda, concurrency)
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
		fmt.Println("\n\nStopping steady state load generator...")
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

	fmt.Printf("\n [Steady State Load Test Summary - Ran for %.2fs]\n", elapsed)
	fmt.Printf("Total Sent: %d | Successfully Queued: %d (%.1f req/sec)\n",
		atomic.LoadUint64(&totalSentCount),
		atomic.LoadUint64(&successCount),
		float64(atomic.LoadUint64(&successCount))/elapsed)
	fmt.Println("\n Task Probability Distribution Generated:")
	fmt.Printf("  • Power Of Two Checks (Light ~33%%):  %d tasks\n", atomic.LoadUint64(&powerOfTwoCount))
	fmt.Printf("  • Prime Calculations (Medium ~33%%):  %d tasks\n", atomic.LoadUint64(&primeTaskCount))
	fmt.Printf("  • Multi-Core Stress (Heavy ~34%%):    %d tasks\n", atomic.LoadUint64(&stressCPUTaskCount))
	fmt.Println("\n Check Master logs/dashboard: Worker pool should remain steady (2-3 containers) without unnecessary scaling!")
}

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
	concurrency       = 10
	arrivalRateLambda = 5.0 // Poisson process mean arrival rate (5 requests per second)
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
	totalSentCount     uint64
	successCount       uint64
)

// generateProbabilisticTask uses Gaussian (Normal) distribution sampling
// centered at mean = 0.8, stddev = 0.15 to probabilistically choose between
// heavy tasks (prime / stress_cpu) and light tasks (is_power_of_two).
func generateProbabilisticTask() TaskPayload {
	// Gaussian sample centered at 0.8
	sample := rand.NormFloat64()*0.15 + 0.8

	if sample >= 0.4 {
		// Heavy CPU Workload (80-90% probability, favoring multi-core stress_cpu)
		if rand.Float64() < 0.7 {
			atomic.AddUint64(&stressCPUTaskCount, 1)
			return TaskPayload{
				Type:  "stress_cpu",
				Value: 5 + rand.Intn(10), // Burn CPU for 5-15 seconds
			}
		} else {
			atomic.AddUint64(&primeTaskCount, 1)
			return TaskPayload{
				Type:  "prime",
				Value: 80000 + rand.Intn(40000), // Find 80,000th to 120,000th prime
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

	// underscore here is basically containing error but since we dont use error here, to avoid unnecessary syntax problem we use _
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

	fmt.Println("[Heavy-Heavy] Starting Continuous Poisson & Gaussian CPU Spike Load Test...")
	fmt.Printf("Running infinitely with Poisson arrival rate (λ=%.1f req/sec, concurrency=%d)...\n", arrivalRateLambda, concurrency)
	fmt.Println("Press Ctrl+C to stop the load test and view final report.")

	client := &http.Client{Timeout: 5 * time.Second}
	startTime := time.Now()

	// Capture Ctrl+C signal for graceful shutdown report
	sigChan := make(chan os.Signal, 1)                   //A channel that receives operating system signals.
	signal.Notify(sigChan, os.Interrupt, syscall.SIGINT) // when user presses ctrl+c, send the notification to sigchan instead of killing the process.

	var wg sync.WaitGroup
	semaphore := make(chan struct{}, concurrency)
	stopChan := make(chan struct{}) //An internal signal channel used to tell the for loop to stop generating requests.

	// Handle graceful shutdown on Ctrl+C
	// A background goroutine that sits waiting for <-sigChan (user pressing Ctrl+C).
	go func() {
		<-sigChan
		fmt.Println("\n\nStopping load test generator...")
		close(stopChan) // Once Ctrl+C is pressed, it executes close(stopChan), signaling the loop below to stop.
	}()

	// Infinite Poisson request generation loop
loop:
	for {
		select {
		case <-stopChan:
			break loop
		default:
			// Calculate Poisson process inter-arrival delay (exponential distribution)

			//rand.ExpFloat64: Returns a random number following an Exponential Distribution. In statistics, the time interval between consecutive Poisson events follows an exponential distribution.
			poissonDelay := time.Duration((rand.ExpFloat64() / arrivalRateLambda) * float64(time.Second))
			time.Sleep(poissonDelay)

			atomic.AddUint64(&totalSentCount, 1)
			wg.Add(1)               // add 1 to the wait group
			semaphore <- struct{}{} // Pause loop if max concurrency is reached
			go func() {
				defer wg.Done()                // Done decrements the wait group
				defer func() { <-semaphore }() // Release semaphore slot when task finishes
				if sendRequest(client) {
					atomic.AddUint64(&successCount, 1)
				}
			}()
		}
	}

	wg.Wait() // This unblocks the wait group, this means all the tasks are done and program can exit.
	elapsed := time.Since(startTime).Seconds()

	fmt.Printf("\n[Continuous Load Test Summary - Ran for %.2fs]\n", elapsed)
	fmt.Printf("Total Sent: %d | Successfully Queued: %d (%.1f req/sec)\n",
		atomic.LoadUint64(&totalSentCount),
		atomic.LoadUint64(&successCount),
		float64(atomic.LoadUint64(&successCount))/elapsed)
	fmt.Println("\n Task Probability Distribution Generated:")
	fmt.Printf("  • Prime Calculations (Heavy):     %d tasks\n", atomic.LoadUint64(&primeTaskCount))
	fmt.Printf("  • Multi-Core Stress (Heavy):     %d tasks\n", atomic.LoadUint64(&stressCPUTaskCount))
	fmt.Printf("  • Power Of Two Checks (Light):    %d tasks\n", atomic.LoadUint64(&powerOfTwoCount))
	fmt.Println("\nCheck Master logs/dashboard: Worker CPU usage should peg near ~100%, triggering scale-up!")
}

//Note :  Key Takeaway: Gaussian determines what task to create, while Poisson determines when to send it!

package orchestrator

import (
	"KubeLite/data"
	"KubeLite/metrics"
	"context"
	"log"
	"math"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"
)

func Orchestrate() {
	ctx := context.Background()
	// Baseline metrics so /metrics is immediately ready on startup
	if _, workers, err := metrics.GetWorkerContainers(ctx); err == nil {
		metrics.UpdateClusterMetrics(0.0, 0.0, len(workers))
	}
	ticker := time.NewTicker(10 * time.Second) //background process that pushes a timestamp in ticker.C pipe every 10 seconds
	defer ticker.Stop()
	var noOfContainer int64
	var rps float64
	var tps float64
	oldRequest := 0
	oldTask := 0

	// Scale-down hysteresis: prevents container churn/flapping during temporary load drops
	var scaleDownCandidateSince time.Time
	scaleDownCooldown := 30 * time.Second
	if rawCooldown := os.Getenv("KUBELITE_SCALE_DOWN_COOLDOWN_SEC"); rawCooldown != "" {
		if sec, err := strconv.Atoi(rawCooldown); err == nil && sec >= 0 {
			scaleDownCooldown = time.Duration(sec) * time.Second
		}
	}
	for { //infinite loop
		//thread goes to sleep if there is nothing in the channel
		<-ticker.C //thread is woken up if something is pushed into ticker.C and ticker.C runs a pop function (<- means pop())
		cli, currentWorkers, err := metrics.GetWorkerContainers(ctx)
		if err != nil {
			log.Printf("Error fetching worker containers: %v", err)
			continue
		}
		Processing_to_main(currentWorkers)
		cpuUsage, err := metrics.GetTotalClusterCPU()
		if err != nil {
			log.Printf("Error fetching cpu data: %v", err)
			continue
		}
		memoryUsage, err := metrics.GetTotalClusterMemory()
		if err != nil {
			log.Printf("Error fetching memory data: %v", err)
			continue
		}
		metrics.UpdateClusterMetrics(cpuUsage, memoryUsage, len(currentWorkers))
		queueLength, err := metrics.GetQueueLength()
		if err != nil {
			log.Printf("Error fetching queue length: %v", err)
			continue
		}
		request, _ := data.GetRequestCount()
		tasks, err := data.GetTotal_Task()
		if err != nil {
			log.Printf("Error fetching tasks: %v", err)
			continue
		}
		rps = float64(request-uint64(oldRequest)) / 10.0
		tps = float64(tasks-int64(oldTask)) / 10.0
		oldRequest = int(request)
		oldTask = int(tasks)
		y, z := getSystemCaps()
		x := CalculateDesiredContainers(
			int64(len(currentWorkers)),
			cpuUsage,
			memoryUsage,
			queueLength,
			rps,
			tps,
		)

		noOfContainer = min(x, min(y, z))
		log.Printf("Metrics - CPU: %.2f%%, Memory: %.2f%%, Queue: %d, RPS: %.2f, TPS: %.2f -> Desired: %d, Target: %d",
			cpuUsage, memoryUsage, queueLength, rps, tps, x, noOfContainer)

		if len(currentWorkers) <= int(noOfContainer) {
			// System is scaling up or maintaining current capacity.
			if !scaleDownCandidateSince.IsZero() {
				log.Printf("Scale-down cooldown canceled: load or target increased (workers: %d, target: %d)",
					len(currentWorkers), noOfContainer)
				scaleDownCandidateSince = time.Time{}
			}
			reqContainers := int(noOfContainer) - len(currentWorkers)
			if reqContainers > 0 {
				Generate(int64(reqContainers))
			}
			metrics.UpdateClusterMetrics(cpuUsage, memoryUsage, int(noOfContainer))
		} else {
			// System wants to scale down (currentWorkers > noOfContainer)
			reqContainers := len(currentWorkers) - int(noOfContainer)
			if scaleDownCooldown == 0 {
				// Immediate scale-down when cooldown is disabled
				Destroy(int64(reqContainers))
				metrics.UpdateClusterMetrics(cpuUsage, memoryUsage, int(noOfContainer))
			} else if scaleDownCandidateSince.IsZero() {
				// First cycle detecting lower load: start the cooldown window
				scaleDownCandidateSince = time.Now()
				log.Printf("Scale-down condition detected (current: %d, target: %d). Starting cooldown window (%v)...",
					len(currentWorkers), noOfContainer, scaleDownCooldown)
				metrics.UpdateClusterMetrics(cpuUsage, memoryUsage, len(currentWorkers))
			} else if time.Since(scaleDownCandidateSince) >= scaleDownCooldown {
				// Cooldown window satisfied without interruption: safely execute scale down
				log.Printf("Scale-down cooldown elapsed (%v). Safely destroying %d worker container(s)...",
					scaleDownCooldown, reqContainers)
				Destroy(int64(reqContainers))
				scaleDownCandidateSince = time.Time{}
				metrics.UpdateClusterMetrics(cpuUsage, memoryUsage, int(noOfContainer))
			} else {
				remaining := scaleDownCooldown - time.Since(scaleDownCandidateSince)
				log.Printf("Scale-down deferred by cooldown window: holding %d workers for another %.1fs to prevent flapping (target: %d)",
					len(currentWorkers), remaining.Seconds(), noOfContainer)
				metrics.UpdateClusterMetrics(cpuUsage, memoryUsage, len(currentWorkers))
			}
		}
		cli.Close()

	}
}
func getSystemCaps() (y int64, z int64) {
	cpus := float64(runtime.NumCPU())         //extracts host's total number of CPU cores
	y = int64(math.Floor((cpus * 0.8) / 0.5)) // at maxt this code uses 80% of total cpus
	totalMem := getHostMemoryMB()
	if totalMem <= 0 {
		z = y
		return y, z
	}
	z = int64(math.Floor((totalMem * 0.8) / 256.0))
	return y, z
}

func getHostMemoryMB() float64 {
	if raw := os.Getenv("KUBELITE_HOST_RAM_MB"); raw != "" {
		if mb, err := strconv.ParseFloat(raw, 64); err == nil && mb > 0 {
			return mb
		}
	}

	data, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return 0
	}
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 2 && fields[0] == "MemTotal:" {
			kb, err := strconv.ParseFloat(fields[1], 64)
			if err == nil {
				return kb / 1024.0
			}
		}
	}
	return 0
}

// ActiveFormulaName identifies the active autoscaling strategy for automated benchmarking.
// When an AI or developer modifies the formula below, update this name so tests and
// benchmark graphs are automatically saved to tests/<ActiveFormulaName>/run_<timestamp>/
const ActiveFormulaName = "balanced_resource_backlog"

// CalculateDesiredContainers computes the optimal number of worker containers (x)
// based on real-time operational telemetry (CPU, Memory, Queue backlog, RPS, and TPS).
// Modify this function to experiment with different autoscaling strategies without
// altering the main Orchestrate loop.
//
// NOTE FOR AI AGENTS & DEVELOPERS:
// If you modify this formula, also update ActiveFormulaName above.
// See tests/README.md for benchmarking instructions.
func CalculateDesiredContainers(
	currentWorkers int64,
	cpuUsage float64,
	memoryUsage float64,
	queueLength int64,
	rps float64,
	tps float64,
) int64 {
	minWorkers := int64(1)
	targetCPU := 70.0
	targetMem := 75.0

	// 1. Resource Demand: Evaluate current CPU & Memory utilization against targets
	resourceDemand := float64(currentWorkers)
	if currentWorkers > 0 {
		cpuRatio := cpuUsage / targetCPU
		memRatio := memoryUsage / targetMem
		resourceDemand = float64(currentWorkers) * math.Max(cpuRatio, memRatio)
	}

	// 2. Traffic Demand: Backlog pressure + net deficit rate
	// Base capacity: 1 container per 5 pending tasks in Redis
	trafficDemand := float64(queueLength) / 5.0
	if rps > tps {
		trafficDemand += (rps - tps) / 2.0
	}

	// 3. Final recommendation: take the higher need between load and queue backlog
	x := int64(math.Ceil(math.Max(resourceDemand, trafficDemand)))
	if x < minWorkers {
		x = minWorkers
	}

	return x
}

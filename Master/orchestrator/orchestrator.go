package orchestrator

import (
	"KubeLite/data"
	"KubeLite/metrics"
	"context"
	"log"
	"math"
	"runtime"
	"syscall"
	"time"
)

func Orchestrate() {
	ctx := context.Background()
	ticker := time.NewTicker(10 * time.Second) //background process that pushes a timestamp in ticker.C pipe every 10 seconds
	defer ticker.Stop()
	var noOfContainer int64
	var rps float64
	var tps float64
	oldRequest := 0
	oldTask := 0
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
			reqContainers := int(noOfContainer) - len(currentWorkers)
			Generate(int64(reqContainers))
		} else {
			reqContianers := len(currentWorkers) - int(noOfContainer)
			Destroy(int64(reqContianers))
		}
		cli.Close()

	}
}
func getSystemCaps() (y int64, z int64) {
	cpus := float64(runtime.NumCPU())         //extracts host's total number of CPU cores
	y = int64(math.Floor((cpus * 0.8) / 0.5)) // at maxt this code uses 80% of total cpus
	var mem syscall.Sysinfo_t                 //syscall.Sysinfo_t extraxts hosts memory
	if err := syscall.Sysinfo(&mem); err != nil {
		z = y
		return y, z
	}
	totalMem := float64(mem.Totalram*uint64(mem.Unit)) / (1024 * 1024) //Total RAM converted to megabytes
	z = int64(math.Floor((totalMem * 0.8) / 256.0))
	return y, z
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


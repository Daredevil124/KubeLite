package orchestrator

import (
	"KubeLite/data"
	"KubeLite/metrics"
	"context"
	"log"
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
	_, currentWorkers, err := metrics.GetWorkerContainers(ctx)
	if err != nil {
		log.Printf("Error fetching worker containers: %v", err)
		continue
	}
	for { //infinite loop
		//thread goes to sleep if there is nothing in the channel
		<-ticker.C //thread is woken up if something is pushed into ticker.C and ticker.C runs a pop function (<- means pop())
		Processing_to_main()
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
		request := data.GetRequestCount()
		tasks, err := data.GetTotal_Task()
		if err != nil {
			log.Printf("Error fetching tasks: %v", err)
			continue
		}
		rps = float64(request-uint64(oldRequest)) / 10.0
		tps = float64(tasks-int64(oldTask)) / 10.0
		oldRequest = int(request)
		oldTask = int(tasks)

		_ = noOfContainer // avoid declared and not used compiler error
		log.Printf("Metrics - CPU: %.2f%%, Memory: %.2f%%, Queue: %d, RPS: %.2f, TPS: %.2f", cpuUsage, memoryUsage, queueLength, rps, tps)

		if len(currentWorkers) <= int(noOfContainer) {
			reqContainers := int(noOfContainer) - len(currentWorkers)
			Generate(int64(reqContainers))
		} else {
			reqContianers := len(currentWorkers) - int(noOfContainer)
			Destroy(int64(reqContianers))
		}

	}
}

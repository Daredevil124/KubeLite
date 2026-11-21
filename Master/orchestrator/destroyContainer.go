package orchestrator

import (
	"KubeLite/metrics"
	"context"
	"log"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/docker/docker/api/types/container"
)

func Destroy(noOfContainer int64) {
	if noOfContainer <= 0 {
		return
	}

	ctx := context.Background()
	cli, containers, err := metrics.GetWorkerContainers(ctx) // gets all running worker containers
	if err != nil {
		log.Printf("failed to create docker client: %v", err)
		return
	}
	defer cli.Close()

	type targetInfo struct {
		id     string
		isIdle bool
	}

	var idleTargets []targetInfo
	var busyTargets []targetInfo

	// Categorize containers by CPU activity (prioritizing idle workers first)
	for _, c := range containers {
		stats, err := cli.ContainerStats(ctx, c.ID, false)
		if err != nil {
			continue
		}
		isIdle := metrics.IsCpuIdle(stats.Body)
		stats.Body.Close()

		if isIdle {
			idleTargets = append(idleTargets, targetInfo{id: c.ID, isIdle: true})
		} else {
			busyTargets = append(busyTargets, targetInfo{id: c.ID, isIdle: false})
		}
	}

	// Select up to noOfContainer to destroy, picking idle containers first
	var toDestroy []targetInfo
	for _, t := range idleTargets {
		if len(toDestroy) >= int(noOfContainer) {
			break
		}
		toDestroy = append(toDestroy, t)
	}
	for _, t := range busyTargets {
		if len(toDestroy) >= int(noOfContainer) {
			break
		}
		toDestroy = append(toDestroy, t)
	}

	if len(toDestroy) == 0 {
		return
	}

	// Stop and remove selected containers concurrently to prevent freezing the orchestrator loop
	defaultBusyTimeout := 10
	if raw := os.Getenv("KUBELITE_STOP_TIMEOUT_SEC"); raw != "" {
		if sec, err := strconv.Atoi(raw); err == nil && sec > 0 {
			defaultBusyTimeout = sec
		}
	}

	var wg sync.WaitGroup
	for _, target := range toDestroy {
		wg.Add(1)
		go func(t targetInfo) {
			defer wg.Done()

			timeout := defaultBusyTimeout
			if t.isIdle {
				timeout = 2 // 2-second grace period for idle workers to handle SIGTERM cleanly
			}

			stopCtx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout+5)*time.Second)
			defer cancel()

			if err := cli.ContainerStop(stopCtx, t.id, container.StopOptions{Timeout: &timeout}); err != nil {
				log.Printf("Failed to stop container %s: %v", t.id, err)
			}
			if err := cli.ContainerRemove(stopCtx, t.id, container.RemoveOptions{Force: true}); err != nil {
				log.Printf("Failed to remove container %s: %v", t.id, err)
			}
		}(target)
	}
	wg.Wait()
}

package metrics

import (
	"context"
	"fmt"
	"sync"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
)

type ClusterMetricsCache struct {
	mu            sync.RWMutex
	CPUPercent    float64
	MemoryPercent float64
	WorkerCount   int
}

var clusterCache ClusterMetricsCache

// UpdateClusterMetrics updates the cached cluster telemetry.
func UpdateClusterMetrics(cpu, memory float64, workerCount int) {
	clusterCache.mu.Lock()
	defer clusterCache.mu.Unlock()
	clusterCache.CPUPercent = cpu
	clusterCache.MemoryPercent = memory
	clusterCache.WorkerCount = workerCount
}

// GetCachedClusterMetrics returns the cached telemetry instantly (< 1ms).
func GetCachedClusterMetrics() (float64, float64, int) {
	clusterCache.mu.RLock()
	defer clusterCache.mu.RUnlock()
	return clusterCache.CPUPercent, clusterCache.MemoryPercent, clusterCache.WorkerCount
}

func GetWorkerContainers(ctx context.Context) (*client.Client, []container.Summary, error) {
	f := filters.NewArgs()
	f.Add("label", "role=worker") //to identify if the docker node is created by this application

	//cli is the object used to talk to docker.
	// fromEnv looks at local linux variables
	// can work with any api version
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create docker client: %w", err)
	}

	//gets all the container from local environment in a list that are running (all:false) and filters he node made by master node
	containers, err := cli.ContainerList(ctx, container.ListOptions{All: false, Filters: f})
	if err != nil {
		cli.Close()
		return nil, nil, fmt.Errorf("failed to list containers: %w", err)
	}

	return cli, containers, nil
}

// GetWorkerCount returns how many containers with label role=worker are currently running.
func GetWorkerCount() (int, error) {
	ctx := context.Background()
	cli, containers, err := GetWorkerContainers(ctx)
	if err != nil {
		return 0, err
	}
	defer cli.Close()
	return len(containers), nil
}

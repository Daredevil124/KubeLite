package metrics

import (
	"context"
	"encoding/json"
	"log"

	"github.com/docker/docker/client"
)

type DockerMemoryStats struct {
	MemoryStats struct {
		Usage uint64            `json:"usage"`
		Limit uint64            `json:"limit"`
		Stats map[string]uint64 `json:"stats"`
	} `json:"memory_stats"`
}

func GetTotalClusterMemory() (float64, error) {
	ctx := context.Background() // control signal, if data does not come after x second, sever the connection, prevents the infinite loop if docker crashes and no replies come

	cli, containers, err := getWorkerContainers(ctx)
	if err != nil {
		return 0, err
	}
	defer cli.Close() //good practice to close

	var totalClusterMemory float64 = 0.0 // total percentage
	for _, val := range containers {
		memPercent, err := fetchContainerMemory(ctx, cli, val.ID)
		if err != nil {
			log.Printf("Failed to fetch stats for %s: %v", val.ID, err)
			continue
		}
		totalClusterMemory += memPercent
	}
	if len(containers) > 0 {
		return totalClusterMemory / float64(len(containers)), nil
	}
	return totalClusterMemory, nil
}

func fetchContainerMemory(ctx context.Context, cli *client.Client, containerID string) (float64, error) {
	stats, err := cli.ContainerStats(ctx, containerID, false) // gets all the stats and then close using false
	if err != nil {
		return 0, err
	}
	defer stats.Body.Close()

	var v DockerMemoryStats
	err = json.NewDecoder(stats.Body).Decode(&v)
	if err != nil {
		return 0, err
	}

	usage := float64(v.MemoryStats.Usage)
	limit := float64(v.MemoryStats.Limit)

	cache := float64(0)
	// Check for cgroup v2 cache key first
	if val, ok := v.MemoryStats.Stats["inactive_file"]; ok {
		cache = float64(val)
	} else if val, ok := v.MemoryStats.Stats["total_inactive_file"]; ok {
		cache = float64(val) // cgroup v1
	}

	if limit <= 0 {
		return 0.0, nil
	}

	memUsed := usage - cache
	if memUsed < 0 {
		memUsed = 0
	}

	return (memUsed / limit) * 100.0, nil
}

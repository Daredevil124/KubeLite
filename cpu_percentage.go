package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
)

type DockerStats struct {
	CPUStats struct {
		CPUUsage struct {
			TotalUsage uint64 `json:"total_usage"`
		} `json:"cpu_usage"`
		SystemCPUUsage uint64 `json:"system_cpu_usage"`
		OnlineCPUs     uint64 `json:"online_cpus"`
	} `json:"cpu_stats"`
	PreCPUStats struct {
		CPUUsage struct {
			TotalUsage uint64 `json:"total_usage"`
		} `json:"cpu_usage"`
		SystemCPUUsage uint64 `json:"system_cpu_usage"`
	} `json:"precpu_stats"`
}

func getTotalClusterCPU() (float64, error) {
	f := filters.NewArgs()
	f.Add("label", "role=worker") //to identify if the docker node is created by this application
	ctx := context.Background()   // control signal, if data does not come after x second, sever the connection, prevents the infinite loop if docker crashes and no replies come
	//cli is the object used to talk to docker.
	// fromEnv looks at local linux variables
	// can work with any api version
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return 0, fmt.Errorf("failed to list containers: %w", err)
	}
	defer cli.Close()                                                                        //good practice to close
	containers, err := cli.ContainerList(ctx, container.ListOptions{All: false, Filters: f}) //gets all the container from local environment in a list that are running (all:false) and filters he node made by master node
	var totalClusterCPU float64 = 0.0                                                        // total percentage
	for _, val := range containers {
		cpuPercent, err := fetchContainerCPU(ctx, cli, val.ID)
		if err != nil {
			log.Printf("Failed to fetch stats for %s: %v", val.ID, err)
			continue
		}
		totalClusterCPU += cpuPercent
	}
	if len(containers) > 0 {
		return totalClusterCPU / float64(len(containers)), nil
	}
	return totalClusterCPU, nil
}
func fetchContainerCPU(ctx context.Context, cli *client.Client, containerID string) (float64, error) {
	stats, err := cli.ContainerStats(ctx, containerID, false) // gets all the stats and then close using false
	if err != nil {
		return 0, err
	}
	defer stats.Body.Close()

	var v DockerStats
	err = json.NewDecoder(stats.Body).Decode(&v)
	if err != nil {
		return 0, err
	}
	cpuDelta := float64(v.CPUStats.CPUUsage.TotalUsage - v.PreCPUStats.CPUUsage.TotalUsage)
	systemDelta := float64(v.CPUStats.SystemCPUUsage - v.PreCPUStats.SystemCPUUsage)
	onlineCPUs := float64(v.CPUStats.OnlineCPUs)
	if systemDelta > 0.0 && cpuDelta > 0.0 {
		return (cpuDelta / systemDelta) * onlineCPUs * 100.0, nil
	}
	return 0.0, nil
}

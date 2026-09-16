package orchestrator

import (
	"KubeLite/metrics"
	"context"
	"log"

	"github.com/docker/docker/api/types/container"
)

func Destroy(noOfContainer int64) {
	ctx := context.Background()
	cli, containers, err := metrics.GetWorkerContainers(ctx) //gets all the containers
	if err != nil {
		log.Printf("failed to create docker client: %v", err)
		return
	}
	defer cli.Close()
	cnt := 0
	for _, c := range containers { //1st loop to kill idle containers
		if cnt >= int(noOfContainer) {
			break
		}
		stats, err := cli.ContainerStats(ctx, c.ID, false) //gets the stat of the current container
		if err != nil {
			continue
		}
		isIdle := metrics.IsCpuIdle(stats.Body)
		stats.Body.Close()

		if isIdle { //checks if the cpu is idle
			timeout := 0
			cli.ContainerStop(ctx, c.ID, container.StopOptions{Timeout: &timeout}) //frees RAM by stoping it
			cli.ContainerRemove(ctx, c.ID, container.RemoveOptions{Force: true})   //frees up disk by removing it
			cnt++
		}
	}
	for _, c := range containers { //second for loop if kill workers that are not idle after they finish their task
		if cnt >= int(noOfContainer) {
			break
		}
		stats, err := cli.ContainerStats(ctx, c.ID, false)
		if err != nil {
			continue
		}
		stats.Body.Close()
		timeout := 30 // time out for 30 seconds before sigkill is sent for workers that are doing tasks
		cli.ContainerStop(ctx, c.ID, container.StopOptions{Timeout: &timeout})
		cli.ContainerRemove(ctx, c.ID, container.RemoveOptions{Force: true})
		cnt++
	}
}

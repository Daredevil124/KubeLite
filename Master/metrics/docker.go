package metrics

import (
	"context"
	"fmt"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
)

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

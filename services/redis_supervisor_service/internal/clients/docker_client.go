package clients

import (
	"context"
	"fmt"
	"time"

	containertypes "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
)

// DockerClient handles communication with the Docker daemon.
type DockerClient struct {
	cli *client.Client
}

// NewDockerClient creates a new Docker client.
func NewDockerClient() (*DockerClient, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("failed to create docker client: %w", err)
	}
	return &DockerClient{cli: cli}, nil
}

// RestartContainer restarts a container by its name.
func (c *DockerClient) RestartContainer(containerName string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// Create StopOptions with a timeout
	timeout := 10
	opts := containertypes.StopOptions{
		Timeout: &timeout,
	}

	// Restart the container
	if err := c.cli.ContainerRestart(ctx, containerName, opts); err != nil {
		return fmt.Errorf("failed to restart container %s: %w", containerName, err)
	}

	return nil
}

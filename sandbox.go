package main

import (
	"bytes"
	"context"
	"fmt"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
)

type TimeoutError struct{}

func (m *TimeoutError) Error() string {
	return "execution timed out"
}

func removeContainer(cli *client.Client, containerID string) {
	fmt.Println("Removing container", containerID)
	cli.ContainerRemove(context.Background(), containerID, container.RemoveOptions{Force: true})
}

func runPHPCode(code string) (string, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return "", err
	}

	fmt.Println("\nCode:")
	fmt.Println("=====")
	fmt.Println(code)
	fmt.Println("=====")

	pidsLimit := int64(3)

	config := &container.Config{
		Image:        "php:alpine",
		Cmd:          []string{"php", "-r", code},
		Tty:          false,
		AttachStdout: true,
		AttachStderr: true,
		User:         "65534:65534", // Run as non-root user
	}

	hostConfig := &container.HostConfig{
		NetworkMode:    "none",
		AutoRemove:     true,
		ReadonlyRootfs: true,
		CapDrop:        []string{"ALL"}, // Drop all capabilities
		SecurityOpt:    []string{"no-new-privileges"},
		// Ulimits: []*units.Ulimit{
		//     // {Name: "nproc", Soft: 2, Hard: 2}, // Limit number of processes
		//     {Name: "cpu", Soft: 1, Hard: 1},   // Limit CPU usage
		// },
		Resources: container.Resources{
			// PidsLimit: int64(10),
			Memory:    40 * 1024 * 1024, // 40 MB
			CPUQuota:  50000,
			PidsLimit: &pidsLimit,
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	resp, err := cli.ContainerCreate(context.Background(), config, hostConfig, nil, nil, "")
	if err != nil {
		return "", err
	}

	containerID := resp.ID

	if err := cli.ContainerStart(ctx, containerID, container.StartOptions{}); err != nil {
		return "", err
	}

	statusCh, errCh := cli.ContainerWait(ctx, containerID, container.WaitConditionNotRunning)

	select {
	case err := <-errCh:
		if err != nil {
			return "", err
		}
	case <-statusCh:
	case <-ctx.Done():
		// If the context times out, stop and remove the container
		removeContainer(cli, containerID)
		return "", &TimeoutError{}
	}

	out, err := cli.ContainerLogs(ctx, containerID, container.LogsOptions{ShowStdout: true, ShowStderr: true})
	if err != nil {
		return "", err
	}

	var stdout, stderr bytes.Buffer
	_, err = stdcopy.StdCopy(&stdout, &stderr, out)
	if err != nil {
		return "", err
	}

	return stdout.String(), nil
}

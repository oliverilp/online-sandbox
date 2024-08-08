package main

import (
	"bytes"
	"context"
	"fmt"
	"log"
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
	log.Println("Removing container manually", containerID)
	cli.ContainerRemove(context.Background(), containerID, container.RemoveOptions{Force: true})
}

func runCode(language string, code string) (string, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return "", err
	}

	var image string
	var cmd []string
	switch language {
	case "php":
		image = "php:8-alpine"
		cmd = []string{"php", "-r", code}
	case "python":
		image = "python:3.12-alpine"
		cmd = []string{"python", "-c", code}
	case "node":
		image = "node:22-alpine"
		cmd = []string{"node", "-e", code}
	default:
		return "", fmt.Errorf("unsupported language: %s", language)
	}

	log.Println("Code:")
	log.Println("=====")
	log.Println(code)
	log.Println("=====")

	pidsLimit := int64(10)

	config := &container.Config{
		Image:        image,
		Cmd:          cmd,
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
			Memory:    25 * 1024 * 1024, // 40 MB
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

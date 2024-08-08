package main

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/limiter"
	"github.com/gofiber/template/html/v2"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
)

type TimeoutError struct{}

func (m *TimeoutError) Error() string {
	return "execution timed out"
}

type PageData struct {
	Output string
}

func main() {
	engine := html.New("./views", ".html")
	app := fiber.New(fiber.Config{
		Views: engine,
	})

	// Rate limiting middleware
	app.Use(limiter.New(limiter.Config{
		Max:        5,
		Expiration: 10 * time.Second,
		KeyGenerator: func(c *fiber.Ctx) string {
			return c.IP()
		},
		LimitReached: func(c *fiber.Ctx) error {
			return c.Status(fiber.StatusTooManyRequests).SendString("Too many requests. Please try again later.")
		},
	}))

	app.Get("/", func(c *fiber.Ctx) error {
		return c.Render("index", fiber.Map{
			"Code": "",
		})
	})

	app.Post("/", func(c *fiber.Ctx) error {
		code := c.FormValue("code")
		output, err := runPHPCode(code)
		if err != nil {
			if _, ok := err.(*TimeoutError); ok {
				output = "Execution timed out after 10 seconds."
			} else {
				output = "Something went wrong while execution your code."
			}
		}

		return c.Render("index", fiber.Map{
			"Output": output,
			"Code":   code,
		})
	})

	log.Println("Starting server at :8080")
	log.Fatal(app.Listen("localhost:8080"))
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

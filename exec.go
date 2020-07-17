package dea

import (
	"context"
	"io"
	"log"
	"os"
	"strings"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/pkg/errors"
)

type ExecParams struct {
	PullImage *string  `json:"pull_image"`
	Image     string   `json:"image"`
	Commands  []string `json:"commands"`
}

func (p *ContainerPool) Exec(params *ExecParams) (*Container, error) {
	//log.Println("executing container, params:")
	//spew.Dump(params)

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Hour)
	defer cancel()

	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, errors.Wrap(err, "failed to connect to docker daemon")
	}

	cnt := NewContainer()
	hostConfig := &container.HostConfig{}

	allow_pull := os.Getenv("ALLOW_PULL")
	if allow_pull == "YES" {
		if params.PullImage != nil {
			reader, err := cli.ImagePull(ctx, *params.PullImage, types.ImagePullOptions{})
			if err != nil {
				return nil, errors.Wrap(err, "failed to pull image to docker daemon")
			}
			io.Copy(cnt.StdOut(), reader)
		}
	}

	cfg := &container.Config{
		Image: params.Image,
		Cmd:   []string{"/bin/bash"},
		//Cmd:   params.Cmd,
	}

	forward_agent := os.Getenv("FORWARD_SSH_AGENT")
	if forward_agent == "YES" {
		sock := os.Getenv("SSH_AUTH_SOCK")
		if sock == "" {
			panic("SSH Agent Forward enabled, but SSH_AUTH_SOCK is not present in Env")
		}
		cfg.Env = []string{"SSH_AUTH_SOCK=/ssh-agent"}

		hostConfig.Mounts = []mount.Mount{
			{
				Type:   mount.TypeBind,
				Source: sock,
				Target: "/ssh-agent",
			},
		}
	}

	resp, err := cli.ContainerCreate(ctx, cfg, hostConfig, nil, "")
	if err != nil {
		return nil, errors.Wrap(err, "failed to create container")
	}
	id := resp.ID
	cnt.ID = id

	p.mutex.Lock()
	p.containers[cnt.ID] = cnt
	p.mutex.Unlock()

	options := types.ContainerAttachOptions{
		Stream: true,
		Stdin:  true,
		Stdout: true,
		Stderr: true,
	}
	log.Println("Attaching to container", id, "...")
	hijacked, err := cli.ContainerAttach(ctx, id, options)
	if err != nil {
		return nil, errors.Wrap(err, "failed to attach to container")
	}
	defer hijacked.Close()

	if err := cli.ContainerStart(ctx, resp.ID, types.ContainerStartOptions{}); err != nil {
		return nil, errors.Wrap(err, "failed to start container")
	}

	// Copy any output to the trace
	stdoutErrCh := make(chan error)
	go func() {
		_, errCopy := stdcopy.StdCopy(cnt.StdOut(), cnt.StdErr(), hijacked.Reader)
		if errCopy != nil {
			stdoutErrCh <- errCopy
		}
	}()

	input := generateScript(params.Commands)
	log.Println("Executing in container", id, "script:\n", input)

	// Write the input to the container and close its STDIN to get it to finish
	stdinErrCh := make(chan error)
	go func() {
		_, errCopy := io.Copy(hijacked.Conn, strings.NewReader(input))
		_ = hijacked.CloseWrite()
		if errCopy != nil {
			log.Println("stdin write error", errCopy)
			stdinErrCh <- errCopy
		}
	}()

	statusCh, waitErrCh := cli.ContainerWait(ctx, resp.ID, container.WaitConditionNotRunning)

	log.Println("waiting for container")

	// Wait until either:
	// - the job is aborted/cancelled/deadline exceeded
	// - stdin has an error
	// - stdout returns an error or nil, indicating the stream has ended and
	//   the container has exited
	select {
	case <-ctx.Done():
		log.Println("context done")
		return nil, errors.New("container execution aborted")
	case err = <-stdinErrCh:
		log.Println("stdin error", err)
		if err != nil {
			return nil, errors.Wrap(err, "container stdin write error")
		}
	case err = <-stdoutErrCh:
		log.Println("stdout error", err)
		if err != nil {
			return nil, errors.Wrap(err, "container stdout read error")
		}
	case err := <-waitErrCh:
		log.Println("wait error", err)
		if err != nil {
			return nil, errors.Wrap(err, "container run/wait error")
		}
	case <-statusCh:
		log.Println("container stopped normally")
	}
	spew.Dump(err)

	//out, err := cli.ContainerLogs(ctx, resp.ID, types.ContainerLogsOptions{ShowStdout: true})
	//if err != nil {
	//return nil, errors.Wrap(err, "container log read error")
	//}

	//stdcopy.StdCopy(cnt.StdOut(), cnt.StdErr(), out)

	//if forward_agent == "YES" {

	//}

	return cnt, nil
}

func generateScript(commands []string) string {
	cmd := "set -eo pipefail\n" + strings.Join(commands, "\n")
	return cmd
}

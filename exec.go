package dea

import (
	"context"
	"io"
	"os"
	"time"

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
	Cmd       []string `json:"cmd"`
}

func (p *ContainerPool) Exec(params *ExecParams) (*Container, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Hour)
	defer cancel()

	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, errors.Wrap(err, "failed to connect to docker daemon")
	}

	cnt := NewContainer()
	hostConfig := &container.HostConfig{}

	allow_pull := os.Getenv("INSECURE_ALLOW_PULL")
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
		Cmd:   params.Cmd,
	}

	forward_agent := os.Getenv("INSECURE_FORWARD_SSH_AGENT")
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

	cnt.ID = resp.ID

	if err := cli.ContainerStart(ctx, resp.ID, types.ContainerStartOptions{}); err != nil {
		return nil, errors.Wrap(err, "failed to start container")
	}

	statusCh, errCh := cli.ContainerWait(ctx, resp.ID, container.WaitConditionNotRunning)
	select {
	case err := <-errCh:
		if err != nil {
			return nil, errors.Wrap(err, "container run/wait error")
		}
	case <-statusCh:
	}

	out, err := cli.ContainerLogs(ctx, resp.ID, types.ContainerLogsOptions{ShowStdout: true})
	if err != nil {
		return nil, errors.Wrap(err, "container log read error")
	}

	stdcopy.StdCopy(cnt.StdOut(), cnt.StdErr(), out)

	p.mutex.Lock()
	p.containers[cnt.ID] = cnt
	p.mutex.Unlock()

	if forward_agent == "YES" {

	}

	return cnt, nil
}

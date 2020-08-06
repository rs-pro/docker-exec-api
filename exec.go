package dea

import (
	"context"
	"io"
	"log"
	"os"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/api/types/volume"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/pkg/errors"
	"github.com/rs-pro/docker-exec-api/config"
)

type ExecParams struct {
	PullImage *string           `json:"pull_image"`
	Image     string            `json:"image"`
	Commands  []string          `json:"commands"`
	Shell     []string          `json:"shell"`
	Volumes   map[string]string `json:"volumes"`
}

func (p *ContainerPool) Exec(params *ExecParams) (*Container, error) {
	var ctx context.Context
	var cancel context.CancelFunc
	if config.Config.Timeout > 0 {
		ctx, cancel = context.WithTimeout(context.Background(), time.Duration(config.Config.Timeout)*time.Second)
		defer cancel()
	} else {
		ctx = context.Background()
	}

	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, errors.Wrap(err, "failed to connect to docker daemon")
	}

	cnt := NewContainer()
	hostConfig := &container.HostConfig{
		Mounts: []mount.Mount{},
	}

	if config.Config.AllowPull {
		if params.PullImage != nil {
			reader, err := cli.ImagePull(ctx, *params.PullImage, types.ImagePullOptions{})
			if err != nil {
				return nil, errors.Wrap(err, "failed to pull image")
			}
			io.Copy(cnt.StdOut(), reader)
		}
	}

	if len(params.Shell) == 0 {
		params.Shell = []string{"/bin/bash"}
	}

	cfg := &container.Config{
		Image: params.Image,
		Cmd:   params.Shell,
		//Cmd:   params.Cmd,
		//AttachStdin:  true,
		//AttachStdout: true,
		//AttachStderr: true,
		//Tty:          true,
		OpenStdin: true,
	}

	if config.Config.ForwardSSHAgent {
		sock := os.Getenv("SSH_AUTH_SOCK")
		if sock == "" {
			return nil, errors.New("SSH Agent Forward enabled, but SSH_AUTH_SOCK is not present in Env (you need to start ssh agent)")
		}
		cfg.Env = []string{"SSH_AUTH_SOCK=/ssh-agent"}

		hostConfig.Mounts = append(hostConfig.Mounts, mount.Mount{
			Type:   mount.TypeBind,
			Source: sock,
			Target: "/ssh-agent",
		})
	}

	if len(params.Volumes) > 0 {
		if !config.Config.AllowVolumes {
			return nil, errors.New("Volumes are disabled. Enable with allow_volumes: true in config.yml")
		}
		for volumeName, volumePath := range params.Volumes {
			v, err := cli.VolumeInspect(ctx, volumeName)
			if err != nil {
				log.Println("volume inspect error", err, "attempt to create volume", volumeName)
				v, err = cli.VolumeCreate(ctx, volume.VolumeCreateBody{
					Driver: "local",
					Name:   volumeName,
				})
				if err != nil {
					return nil, errors.Wrap(err, "failed to create volume")
				}
			}
			hostConfig.Mounts = append(hostConfig.Mounts, mount.Mount{
				Type:   mount.TypeVolume,
				Source: v.Name,
				Target: volumePath,
			})
		}
	}

	resp, err := cli.ContainerCreate(ctx, cfg, hostConfig, nil, nil, "")
	if err != nil {
		return nil, errors.Wrap(err, "failed to create container")
	}
	id := resp.ID
	cnt.ID = id

	p.mutex.Lock()
	p.containers[cnt.ID] = cnt
	p.mutex.Unlock()

	log.Println("Attaching to container", id, "...")

	options := types.ContainerAttachOptions{
		// TODO - no logs are returned from attach ? should work but doesn't
		//Logs:   true,
		Stream: true,
		Stdin:  true,
		Stdout: true,
		Stderr: true,
	}
	hijacked, err := cli.ContainerAttach(ctx, id, options)
	if err != nil {
		return nil, errors.Wrap(err, "failed to attach to container")
	}
	defer hijacked.Close()

	// Copy any output to the trace
	//stdoutErrCh := make(chan error)
	//go func() {
	//_, errCopy := stdcopy.StdCopy(cnt.StdOut(), cnt.StdErr(), hijacked.Reader)
	//if errCopy != nil {
	//log.Println("container attach stdcopy error", errCopy)
	//stdoutErrCh <- errCopy
	//} else {
	//log.Println("container attach stdcopy returned without error")
	//}
	//}()

	if err := cli.ContainerStart(ctx, resp.ID, types.ContainerStartOptions{}); err != nil {
		return nil, errors.Wrap(err, "failed to start container")
	}

	commands := generateScript(params.Commands)

	// Write the input to the container and close its STDIN to get it to finish
	stdinErrCh := make(chan error)
	go func() {
		_, errWrite := hijacked.Conn.Write([]byte(bashPre))
		if errWrite != nil {
			stdinErrCh <- errWrite
			return
		}
		_, errWrite = hijacked.Conn.Write([]byte(bashTrapShellScript))
		if errWrite != nil {
			stdinErrCh <- errWrite
			return
		}

		for _, cmd := range commands {
			cnt.StartCommand(cmd)

			cnt.Cond.L.Lock()
			processed := PrepareCommand(cmd)
			log.Println("command:", cmd)
			//log.Println("writing command:", processed)
			_, errWrite := hijacked.Conn.Write([]byte(processed))
			if errWrite != nil {
				log.Println("stdin write error", errWrite)
				stdinErrCh <- errWrite
			} else {
				cnt.Cond.Broadcast()
			}
			cnt.Cond.L.Unlock()

			cmd := cnt.LastCommand()
			for cmd.ExitCode == nil {
				cnt.StdinCond.L.Lock()
				cnt.StdinCond.Wait()

				if cmd.ExitCode == nil || *cmd.ExitCode != 0 {
					log.Println("bad exit code, stopping")
					stdinErrCh <- errors.New("exit code error")
					cnt.StdinCond.L.Unlock()
					return
				} else {
					cnt.StdinCond.L.Unlock()
				}
			}
		}

		//errClose := hijacked.CloseWrite()
		//if errClose != nil {
		//log.Println("stdin CloseWrite error", errClose)
		//stdinErrCh <- errClose
		//}
	}()

	statusCh, waitErrCh := cli.ContainerWait(ctx, resp.ID, container.WaitConditionNotRunning)

	log.Println("waiting for container")

	out, err := cli.ContainerLogs(ctx, resp.ID, types.ContainerLogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Follow:     true,
		Details:    true,
	})
	if err != nil {
		return nil, errors.Wrap(err, "container log read error")
	}

	logsErrCh := make(chan error)
	go func() {
		_, errLogs := stdcopy.StdCopy(cnt.StdOut(), cnt.StdErr(), out)
		if errLogs != nil {
			log.Println("logs stdcopy err")
			logsErrCh <- errLogs
		}
	}()

	// Wait until either:
	// - the job is aborted/cancelled/deadline exceeded
	// - stdin has an error
	// - stdout returns an error or nil, indicating the stream has ended and
	//   the container has exited
	for {
		select {
		case <-ctx.Done():
			log.Println("context done")
			return nil, errors.New("container execution aborted")
		case err = <-stdinErrCh:
			log.Println("stdin error", err)
			if err != nil {
				return nil, errors.Wrap(err, "container stdin write error")
			}
		case err = <-logsErrCh:
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
		if err != nil {
			break
		}
	}
	cnt.Stopped = true

	return cnt, nil
}

func generateScript(commands []string) []string {
	systemCommands := []string{}
	if config.Config.ForwardSSHAgent {
		systemCommands = append(systemCommands, "mkdir ~/.ssh")
		for _, key := range config.Config.SSHHostKeys {
			systemCommands = append(systemCommands, "echo '"+key+"' > ~/.ssh/known_hosts")
		}
	}
	//systemCommands = append(systemCommands,
	//"cat ~/.ssh/known_hosts",
	//"ssh-add -L",
	//"mkdir /data",
	//"cd /data",
	//)

	systemCommands = append(systemCommands, commands...)
	return systemCommands
	//cmd := strings.Join(systemCommands, "\n") + "\n" + strings.Join(commands, "\n")
	//return cmd
}

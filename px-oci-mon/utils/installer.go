package utils

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/client"
)

// DockerInstaller is a Docker client specialized for Container installation
type DockerInstaller struct {
	auth string
	ctx  context.Context
	cli  *client.Client
}

// NewDockerInstaller creates an instance of the DockerInstaller
func NewDockerInstaller(user, pass string) (*DockerInstaller, error) {
	auth, ctx := "", context.Background()

	// NOTE: see https://docs.docker.com/engine/api/v1.26/#section/Versioning
	cli, err := client.NewClient("unix:///var/run/docker.sock", "1.23", nil, nil)
	if err != nil {
		return nil, err
	}

	cli.NegotiateAPIVersion(ctx)

	if user != "" {
		js := fmt.Sprintf(`{"username":%q,"password":%q}`, user, pass)
		auth = base64.StdEncoding.EncodeToString([]byte(js))
	}
	return &DockerInstaller{
		auth: auth,
		ctx:  ctx,
		cli:  cli,
	}, nil
}

// PullImage pulls the image of a given name
func (di *DockerInstaller) PullImage(name string) error {
	opts := types.ImagePullOptions{RegistryAuth: di.auth}
	out, err := di.cli.ImagePull(di.ctx, name, opts)
	if err != nil {
		return err
	}
	io.Copy(os.Stdout, out)
	return nil
}

// RunOnce will create container, run it, wait until it's finished, and finally remove it.
func (di *DockerInstaller) RunOnce(name, cntr string, args, mounts []string) error {
	contConf := container.Config{
		Image:        name,
		Cmd:          args,
		AttachStdin:  false,
		AttachStdout: true,
		AttachStderr: true,
		Tty:          false,
	}

	// handle mounts
	hostConf := container.HostConfig{}
	if len(mounts) > 0 {
		hostConf.Mounts = make([]mount.Mount, len(mounts))
		for i, m := range mounts {
			var mnt mount.Mount
			parts := strings.Split(m, ":")
			switch len(parts) {
			case 1:
				// parsed "/opt/pwx" -style mount
				mnt = mount.Mount{
					Source: m,
					Target: m,
					Type:   mount.TypeBind,
				}
			case 2:
				// parsed "/opt/pwx:/opt/pwx" -style mount
				mnt = mount.Mount{
					Source: parts[0],
					Target: parts[1],
					Type:   mount.TypeBind,
				}
			default:
				// parsed "/opt/pwx:/opt/pwx:shared,ro" -style mount
				return fmt.Errorf("INTERNAL ERROR: do not handle propagated mounts (%s)", m)
			}
			hostConf.Mounts[i] = mnt
		}
	}

	logrus.Infof("Removing old container %s (if any)", cntr)
	err := di.cli.ContainerRemove(di.ctx, cntr, types.ContainerRemoveOptions{
		RemoveVolumes: true,
		Force:         true,
	})
	logrus.WithError(err).Debug("Old container removed")

	logrus.Info("Creating container from image ", name)
	resp, err := di.cli.ContainerCreate(di.ctx, &contConf, &hostConf, nil, cntr)
	if err != nil {
		return fmt.Errorf("Could not create container %s: %s", name, err)
	}

	logrus.Infof("Starting container %s [%s]", resp.ID, name)
	if err := di.cli.ContainerStart(di.ctx, resp.ID, types.ContainerStartOptions{}); err != nil {
		return fmt.Errorf("Could not start container %s [%s]: %s", resp.ID, name, err)
	}

	// after this point, we want to always dump the logs, and remove container only in all OK
	var retError error

	logrus.Infof("Waiting for container %s [%s]", resp.ID, name)
	resultC, errC := di.cli.ContainerWait(di.ctx, resp.ID, container.WaitConditionNotRunning)
	select {
	case result := <-resultC:
		if result.StatusCode != 0 {
			retError = fmt.Errorf("Expected status code '0', got %d", result.StatusCode)
		}
	case err := <-errC:
		retError = fmt.Errorf("Error while running container %s [%s]: %s", resp.ID, name, err)
	}

	logrus.Infof("Logs for container %s [%s]", resp.ID, name)
	out, err := di.cli.ContainerLogs(di.ctx, resp.ID, types.ContainerLogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Details:    true,
	})
	if err != nil {
		retError = fmt.Errorf("Could not get logs for container %s [%s]: %s", resp.ID, name, err)
	}
	io.Copy(os.Stdout, out)

	if retError == nil {
		logrus.Infof("Removing container %s [%s]", resp.ID, name)
		err = di.cli.ContainerRemove(di.ctx, resp.ID, types.ContainerRemoveOptions{
			RemoveVolumes: true,
			Force:         true,
		})
		if err != nil {
			retError = fmt.Errorf("Could not remove container %s [%s]: %s", resp.ID, name, err)
		}
	}

	return retError
}

// ExtractConfig extracts the containers configuration
func (di *DockerInstaller) ExtractConfig(id string) (*SimpleContainerConfig, error) {
	scc := SimpleContainerConfig{}

	cconf, err := di.cli.ContainerInspect(di.ctx, id)
	if err != nil {
		return nil, fmt.Errorf("Error inspecting container '%s': %s", id, err)
	}
	logrus.Debugf("COnFIG:%+v", cconf)

	// Copy arguments
	scc.Args = make([]string, len(cconf.Args))
	copy(scc.Args, cconf.Args)

	// Copy mounts
	scc.Mounts = formatMounts(cconf.Mounts)

	// Copy ENV
	scc.Env = make([]string, len(cconf.Config.Env))
	copy(scc.Env, cconf.Config.Env)

	// Copy LABELS
	scc.Labels = make(map[string]string)
	for k, v := range cconf.Config.Labels {
		scc.Labels[k] = v
	}

	return &scc, nil
}

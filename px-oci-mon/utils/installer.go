package utils

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/tlsconfig"
	"path/filepath"
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
	cli, err := client.NewEnvClient()
	if err != nil {
		return nil, err
	}

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

// NewDockerInstallerDirect creates an instance of the DockerInstaller using a "direct" Docker client invocation
func NewDockerInstallerDirect(apiVersion, user, pass string) (*DockerInstaller, error) {
	auth, ctx := "", context.Background()

	var httpClient *http.Client
	if dockerCertPath := os.Getenv("DOCKER_CERT_PATH"); dockerCertPath != "" {
		options := tlsconfig.Options{
			CAFile:             filepath.Join(dockerCertPath, "ca.pem"),
			CertFile:           filepath.Join(dockerCertPath, "cert.pem"),
			KeyFile:            filepath.Join(dockerCertPath, "key.pem"),
			InsecureSkipVerify: os.Getenv("DOCKER_TLS_VERIFY") == "",
		}
		tlsc, err := tlsconfig.Client(options)
		if err != nil {
			return nil, err
		}

		httpClient = &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: tlsc,
			},
		}
	}

	cli, err := client.NewClient(client.DefaultDockerHost, apiVersion, &httpClient, nil)

	if err != nil {
		return nil, err
	}

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
